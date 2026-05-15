package domain

import "time"

// ====================== Activity DTOs ======================

// ActivityResponse for activity endpoints
type ActivityResponse struct {
	ID          string    `json:"id"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	IPAddress   string    `json:"ip_address"`
	DeviceInfo  string    `json:"device_info"`
	CreatedAt   time.Time `json:"created_at"`
}

// ActivityQuery for filtering activities
type ActivityQuery struct {
	Action    string     `form:"action"`
	StartDate *time.Time `form:"start_date"`
	EndDate   *time.Time `form:"end_date"`
	Limit     int        `form:"limit,default=50"`
	Offset    int        `form:"offset,default=0"`
}

// ActivitySummary for activity summary
type ActivitySummary struct {
	TotalActivities    int            `json:"total_activities"`
	ActionBreakdown    map[string]int `json:"action_breakdown"`
	MostActiveDay      string         `json:"most_active_day"`
	AveragePerDay      float64        `json:"average_per_day"`
	UniqueIPAddresses  int            `json:"unique_ip_addresses"`
	UniqueDevices      int            `json:"unique_devices"`
	AnalysisPeriodDays int            `json:"analysis_period_days"`
}
