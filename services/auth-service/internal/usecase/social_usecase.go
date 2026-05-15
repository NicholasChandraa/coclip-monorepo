package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"auth-service/pkg/crypto"
	redisClient "auth-service/pkg/redis"

	"github.com/google/uuid"
)

// SocialUseCase handles OAuth flows and token management for social platforms.
type SocialUseCase interface {
	GetYouTubeOAuthURL(ctx context.Context, userID uuid.UUID) (string, error)
	HandleYouTubeCallback(ctx context.Context, code, state string) error
	GetTikTokOAuthURL(ctx context.Context, userID uuid.UUID) (string, error)
	HandleTikTokCallback(ctx context.Context, code, state string) error
	GetConnectedAccounts(ctx context.Context, userID uuid.UUID) ([]domain.SocialAccountResponse, error)
	DisconnectAccount(ctx context.Context, userID uuid.UUID, platform string) error
	GetValidToken(ctx context.Context, userID uuid.UUID, platform string) (string, time.Time, string, string, error)
}

type socialUseCase struct {
	socialRepo repository.SocialAccountRepository
	redis      *redisClient.Client // wrapper that holds the raw *redis.Client
	config     *config.Config
}

// NewSocialUseCase creates a new SocialUseCase with the given dependencies.
// redisWrapper is the project-standard Redis wrapper from pkg/redis.
func NewSocialUseCase(
	socialRepo repository.SocialAccountRepository,
	redisWrapper *redisClient.Client,
	cfg *config.Config,
) SocialUseCase {
	return &socialUseCase{
		socialRepo: socialRepo,
		redis:      redisWrapper,
		config:     cfg,
	}
}

// GetYouTubeOAuthURL generates a Google OAuth2 authorization URL and stores the
// CSRF state token in Redis with a 10-minute TTL so the callback can validate it.
func (u *socialUseCase) GetYouTubeOAuthURL(ctx context.Context, userID uuid.UUID) (string, error) {
	state := uuid.New().String()

	// Store state → userID mapping; GetDel in the callback makes it single-use.
	if err := u.redis.Set(ctx, "social_oauth_state:"+state, userID.String(), 10*time.Minute); err != nil {
		return "", err
	}

	params := url.Values{
		"client_id":     {u.config.Social.YouTube.ClientID},
		"redirect_uri":  {u.config.Social.YouTube.RedirectURI},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/youtube"},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode(), nil
}

// HandleYouTubeCallback validates the OAuth state, exchanges the authorization code
// for tokens, fetches the user's YouTube channel info, encrypts the tokens with
// AES-256-GCM, and upserts the SocialAccount row.
func (u *socialUseCase) HandleYouTubeCallback(ctx context.Context, code, state string) error {
	// GetDel is an atomic get-and-delete — the state key is consumed on first use.
	// We access the raw *redis.Client via GetRawClient() because the wrapper does
	// not expose GetDel directly.
	userIDStr, err := u.redis.GetRawClient().GetDel(ctx, "social_oauth_state:"+state).Result()
	if err != nil {
		return errors.New("invalid or expired OAuth state")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return errors.New("invalid user_id in state")
	}

	tokens, err := u.exchangeCode(ctx, code)
	if err != nil {
		return err
	}

	channelID, channelTitle, err := u.getChannelInfo(ctx, tokens.AccessToken)
	if err != nil {
		return err
	}

	encAccess, err := crypto.EncryptAES256GCM(u.config.Social.EncryptionKey, tokens.AccessToken)
	if err != nil {
		return err
	}
	encRefresh, err := crypto.EncryptAES256GCM(u.config.Social.EncryptionKey, tokens.RefreshToken)
	if err != nil {
		return err
	}

	account := &domain.SocialAccount{
		ID:               uuid.New(),
		UserID:           userID,
		Platform:         "youtube",
		AccessToken:      encAccess,
		RefreshToken:     encRefresh,
		TokenExpiry:      time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		Scope:            tokens.Scope,
		PlatformUserID:   channelID,
		PlatformUsername: channelTitle,
	}
	return u.socialRepo.Upsert(ctx, account)
}

// generateCodeVerifier generates a random 43-character string for PKCE.
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Base64Url encode without padding
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateCodeChallenge generates a SHA256 code challenge from the code verifier.
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GetTikTokOAuthURL generates a TikTok OAuth authorization URL and stores the
// CSRF state token and code_verifier in Redis with a 10-minute TTL.
func (u *socialUseCase) GetTikTokOAuthURL(ctx context.Context, userID uuid.UUID) (string, error) {
	state := uuid.New().String()
	
	verifier, err := generateCodeVerifier()
	if err != nil {
		return "", err
	}
	challenge := generateCodeChallenge(verifier)

	// Store both userID and the code_verifier
	stateData := fmt.Sprintf("%s|%s", userID.String(), verifier)

	if err := u.redis.Set(ctx, "social_oauth_state:"+state, stateData, 10*time.Minute); err != nil {
		return "", err
	}

	params := url.Values{
		"client_key":    {u.config.Social.TikTok.ClientKey},
		"response_type": {"code"},
		"scope":         {"user.info.basic,video.publish"},
		"redirect_uri":  {u.config.Social.TikTok.RedirectURI},
		"state":         {state},
		"code_challenge": {challenge},
		"code_challenge_method": {"S256"},
	}
	return "https://www.tiktok.com/v2/auth/authorize/?" + params.Encode(), nil
}

// HandleTikTokCallback validates the OAuth state, exchanges the authorization code
// for tokens, fetches the user's TikTok info, encrypts the tokens with
// AES-256-GCM, and upserts the SocialAccount row.
func (u *socialUseCase) HandleTikTokCallback(ctx context.Context, code, state string) error {
	stateDataStr, err := u.redis.GetRawClient().GetDel(ctx, "social_oauth_state:"+state).Result()
	if err != nil {
		return errors.New("invalid or expired OAuth state")
	}

	parts := strings.Split(stateDataStr, "|")
	if len(parts) != 2 {
		return errors.New("invalid state data format")
	}
	userIDStr := parts[0]
	codeVerifier := parts[1]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return errors.New("invalid user_id in state")
	}

	tokens, err := u.exchangeTikTokCode(ctx, code, codeVerifier)
	if err != nil {
		return err
	}

	openID, displayName, err := u.getTikTokUserInfo(ctx, tokens.AccessToken)
	if err != nil {
		return err
	}

	encAccess, err := crypto.EncryptAES256GCM(u.config.Social.EncryptionKey, tokens.AccessToken)
	if err != nil {
		return err
	}
	encRefresh, err := crypto.EncryptAES256GCM(u.config.Social.EncryptionKey, tokens.RefreshToken)
	if err != nil {
		return err
	}

	account := &domain.SocialAccount{
		ID:               uuid.New(),
		UserID:           userID,
		Platform:         "tiktok",
		AccessToken:      encAccess,
		RefreshToken:     encRefresh,
		TokenExpiry:      time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		Scope:            tokens.Scope,
		PlatformUserID:   openID,
		PlatformUsername: displayName,
	}
	return u.socialRepo.Upsert(ctx, account)
}

// GetConnectedAccounts returns the public-safe view of all social accounts linked
// to the given user. Token fields are intentionally excluded from the response type.
func (u *socialUseCase) GetConnectedAccounts(ctx context.Context, userID uuid.UUID) ([]domain.SocialAccountResponse, error) {
	accounts, err := u.socialRepo.FindAllByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.SocialAccountResponse, len(accounts))
	for i, a := range accounts {
		result[i] = domain.SocialAccountResponse{
			ID:               a.ID.String(),
			Platform:         a.Platform,
			PlatformUserID:   a.PlatformUserID,
			PlatformUsername: a.PlatformUsername,
			ConnectedAt:      a.CreatedAt,
		}
	}
	return result, nil
}

// DisconnectAccount removes the stored social account row for the given user and platform.
func (u *socialUseCase) DisconnectAccount(ctx context.Context, userID uuid.UUID, platform string) error {
	return u.socialRepo.DeleteByUserAndPlatform(ctx, userID, platform)
}

// GetValidToken returns a valid plaintext access token for the given user+platform.
// If the stored token expires within 5 minutes it is proactively refreshed via
// the Google token endpoint before being returned. The newly-issued tokens are
// is re-encrypted and persisted so subsequent calls do not trigger another refresh.
func (u *socialUseCase) GetValidToken(ctx context.Context, userID uuid.UUID, platform string) (string, time.Time, string, string, error) {
	account, err := u.socialRepo.FindByUserAndPlatform(ctx, userID, platform)
	if err != nil {
		return "", time.Time{}, "", "", errors.New("account not connected")
	}

	accessToken, err := crypto.DecryptAES256GCM(u.config.Social.EncryptionKey, account.AccessToken)
	if err != nil {
		return "", time.Time{}, "", "", err
	}

	// Return the current token if it is still valid for more than 5 minutes.
	if time.Now().Add(5 * time.Minute).Before(account.TokenExpiry) {
		return accessToken, account.TokenExpiry, account.PlatformUserID, account.PlatformUsername, nil
	}

	// Token is expired or about to expire — refresh it.
	refreshToken, err := crypto.DecryptAES256GCM(u.config.Social.EncryptionKey, account.RefreshToken)
	if err != nil {
		return "", time.Time{}, "", "", err
	}

	var newAccessToken, newRefreshToken string
	var newExpiry time.Time

	switch platform {
	case "youtube":
		newTokens, err := u.refreshYouTubeToken(ctx, refreshToken)
		if err != nil {
			return "", time.Time{}, "", "", err
		}
		newAccessToken = newTokens.AccessToken
		newRefreshToken = newTokens.RefreshToken
		newExpiry = time.Now().Add(time.Duration(newTokens.ExpiresIn) * time.Second)
	case "tiktok":
		newTokens, err := u.refreshTikTokToken(ctx, refreshToken)
		if err != nil {
			return "", time.Time{}, "", "", err
		}
		newAccessToken = newTokens.AccessToken
		newRefreshToken = newTokens.RefreshToken
		newExpiry = time.Now().Add(time.Duration(newTokens.ExpiresIn) * time.Second)
	default:
		return "", time.Time{}, "", "", errors.New("unsupported platform for token refresh")
	}

	encAccess, _ := crypto.EncryptAES256GCM(u.config.Social.EncryptionKey, newAccessToken)

	// Keep existing if refresh was omitted from response
	encRefresh := account.RefreshToken
	if newRefreshToken != "" {
		encRefresh, _ = crypto.EncryptAES256GCM(u.config.Social.EncryptionKey, newRefreshToken)
	}

	// Persist on a best-effort basis — a failure here is non-fatal; the caller
	// still receives a valid token for the current request.
	account.AccessToken = encAccess
	account.RefreshToken = encRefresh
	account.TokenExpiry = newExpiry
	if err := u.socialRepo.Upsert(ctx, account); err != nil {
		return "", time.Time{}, "", "", err
	}

	return newAccessToken, newExpiry, account.PlatformUserID, account.PlatformUsername, nil
}

// --- Internal helpers ---

// googleTokenResponse mirrors the JSON body returned by the Google OAuth2 token endpoint.
type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
}

// exchangeCode performs the authorization-code → token exchange against Google's
// token endpoint. ctx is forwarded to the HTTP request so callers can cancel it.
func (u *socialUseCase) exchangeCode(ctx context.Context, code string) (*googleTokenResponse, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {u.config.Social.YouTube.ClientID},
		"client_secret": {u.config.Social.YouTube.ClientSecret},
		"redirect_uri":  {u.config.Social.YouTube.RedirectURI},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokens googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	if tokens.Error != "" {
		return nil, errors.New("Google token exchange error: " + tokens.Error)
	}
	if tokens.AccessToken == "" {
		return nil, errors.New("no access_token in Google response")
	}
	return &tokens, nil
}

// refreshYouTubeToken exchanges a refresh token for a new access token via Google's
// token endpoint. ctx is forwarded so the caller can apply timeouts.
func (u *socialUseCase) refreshYouTubeToken(ctx context.Context, refreshToken string) (*googleTokenResponse, error) {
	data := url.Values{
		"refresh_token": {refreshToken},
		"client_id":     {u.config.Social.YouTube.ClientID},
		"client_secret": {u.config.Social.YouTube.ClientSecret},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokens googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	if tokens.Error != "" {
		return nil, errors.New("Google token refresh error: " + tokens.Error)
	}
	return &tokens, nil
}

// getChannelInfo queries the YouTube Data API for the authenticated user's channel.
// It returns the channel's resource ID and display title.
func (u *socialUseCase) getChannelInfo(ctx context.Context, accessToken string) (id, title string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}
	if len(result.Items) == 0 {
		return "", "", errors.New("no YouTube channel found for this account")
	}
	return result.Items[0].ID, result.Items[0].Snippet.Title, nil
}

type tikTokTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	OpenID       string `json:"open_id"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (u *socialUseCase) exchangeTikTokCode(ctx context.Context, code, codeVerifier string) (*tikTokTokenResponse, error) {
	data := url.Values{
		"client_key":    {u.config.Social.TikTok.ClientKey},
		"client_secret": {u.config.Social.TikTok.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {u.config.Social.TikTok.RedirectURI},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.tiktokapis.com/v2/oauth/token/",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		AccessToken      string `json:"access_token"`
		ExpiresIn        int    `json:"expires_in"`
		OpenID           string `json:"open_id"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New("TikTok token exchange error: " + result.Error + " - " + result.ErrorDescription)
	}
	if result.AccessToken == "" {
		return nil, errors.New("no access_token in TikTok response")
	}

	return &tikTokTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		OpenID:       result.OpenID,
		Scope:        result.Scope,
	}, nil
}

func (u *socialUseCase) refreshTikTokToken(ctx context.Context, refreshToken string) (*tikTokTokenResponse, error) {
	data := url.Values{
		"client_key":    {u.config.Social.TikTok.ClientKey},
		"client_secret": {u.config.Social.TikTok.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.tiktokapis.com/v2/oauth/token/",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		AccessToken      string `json:"access_token"`
		ExpiresIn        int    `json:"expires_in"`
		OpenID           string `json:"open_id"`
		RefreshToken     string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New("TikTok token refresh error: " + result.Error + " - " + result.ErrorDescription)
	}
	
	return &tikTokTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		OpenID:       result.OpenID,
	}, nil
}

func (u *socialUseCase) getTikTokUserInfo(ctx context.Context, accessToken string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://open.tiktokapis.com/v2/user/info/?fields=open_id,username", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			User struct {
				OpenID   string `json:"open_id"`
				Username string `json:"username"`
			} `json:"user"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}
	if result.Error.Code != "ok" && result.Error.Code != "" {
		return "", "", errors.New("TikTok user info error: " + result.Error.Message)
	}
	return result.Data.User.OpenID, result.Data.User.Username, nil
}
