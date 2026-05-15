package handler

import (
	"net/http"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	"auth-service/internal/middleware"
	"auth-service/internal/usecase"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SocialHandler handles OAuth and social account management endpoints.
// It sits in the HTTP layer: it parses requests, delegates to SocialUseCase,
// and formats responses. It has no direct knowledge of database or OAuth internals.
type SocialHandler struct {
	socialUC usecase.SocialUseCase
	config   *config.Config
}

// NewSocialHandler creates a new SocialHandler with the given dependencies.
func NewSocialHandler(socialUC usecase.SocialUseCase, cfg *config.Config) *SocialHandler {
	return &SocialHandler{
		socialUC: socialUC,
		config:   cfg,
	}
}

// StartYouTubeOAuth handles GET /api/v1/social/auth/youtube/start
// JWT required. Generates a Google OAuth2 authorization URL containing a
// single-use CSRF state token and returns it to the client for redirect.
func (h *SocialHandler) StartYouTubeOAuth(c *gin.Context) {
	// user_id is stored in context as a string by AuthMiddleware; parse to UUID.
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in token"})
		return
	}

	authURL, err := h.socialUC.GetYouTubeOAuthURL(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OAuth URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// YouTubeCallback handles GET /api/v1/social/auth/youtube/callback
// Public — Google redirects the browser here after user grants consent.
// Validates the state token, exchanges the authorization code, persists the
// encrypted tokens, then redirects the browser back to the frontend.
func (h *SocialHandler) YouTubeCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.Redirect(http.StatusFound, h.config.Social.FrontendURL+"/settings?error=missing_params")
		return
	}

	if err := h.socialUC.HandleYouTubeCallback(c.Request.Context(), code, state); err != nil {
		log := logger.GetLoggerFromGinContext(c)
		log.Error().Err(err).Msg("YouTube OAuth callback failed")
		c.Redirect(http.StatusFound, h.config.Social.FrontendURL+"/settings?error=youtube_connect_failed")
		return
	}

	c.Redirect(http.StatusFound, h.config.Social.FrontendURL+"/settings?connected=youtube")
}

// StartTikTokOAuth handles GET /api/v1/social/auth/tiktok/start
func (h *SocialHandler) StartTikTokOAuth(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in token"})
		return
	}

	authURL, err := h.socialUC.GetTikTokOAuthURL(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OAuth URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// TikTokCallback handles GET /api/v1/social/auth/tiktok/callback
func (h *SocialHandler) TikTokCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.Redirect(http.StatusFound, h.config.Social.FrontendURL+"/settings?error=missing_params")
		return
	}

	if err := h.socialUC.HandleTikTokCallback(c.Request.Context(), code, state); err != nil {
		log := logger.GetLoggerFromGinContext(c)
		log.Error().Err(err).Msg("TikTok OAuth callback failed")
		c.Redirect(http.StatusFound, h.config.Social.FrontendURL+"/settings?error=tiktok_connect_failed")
		return
	}

	c.Redirect(http.StatusFound, h.config.Social.FrontendURL+"/settings?connected=tiktok")
}

// GetAccounts handles GET /api/v1/social/accounts
// JWT required. Returns all connected social accounts for the authenticated user.
// Token fields are intentionally excluded from the response.
func (h *SocialHandler) GetAccounts(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in token"})
		return
	}

	accounts, err := h.socialUC.GetConnectedAccounts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch accounts"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

// DisconnectAccount handles DELETE /api/v1/social/accounts/:platform
// JWT required. Removes the stored OAuth tokens for the given platform so the
// user's account is no longer linked.
func (h *SocialHandler) DisconnectAccount(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in token"})
		return
	}

	platform := c.Param("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform parameter is required"})
		return
	}

	if err := h.socialUC.DisconnectAccount(c.Request.Context(), userID, platform); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "disconnected"})
}

// GetInternalToken handles GET /api/v1/internal/social/token/:user_id/:platform
// Service-token protected (X-Service-Token header). Called by the engine service
// to retrieve a valid plaintext access token before uploading a clip to a platform.
// The usecase transparently refreshes and persists a new token if the stored one
// is within 5 minutes of expiry.
func (h *SocialHandler) GetInternalToken(c *gin.Context) {
	userIDStr := c.Param("user_id")
	platform := c.Param("platform")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	accessToken, expiry, openID, username, err := h.socialUC.GetValidToken(c.Request.Context(), userID, platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain.InternalTokenResponse{
		AccessToken:      accessToken,
		TokenExpiry:      expiry,
		OpenID:           openID,
		PlatformUsername: username,
	})
}
