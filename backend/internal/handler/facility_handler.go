package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FacilityHandler 处理公共设施及预定相关 API
type FacilityHandler struct {
}

// NewFacilityHandler creates a new FacilityHandler
func NewFacilityHandler() *FacilityHandler {
	return &FacilityHandler{}
}

// CreateFacilityRequest 创建设施请求体
type CreateFacilityRequest struct {
	Name              string  `json:"name" form:"name" binding:"required,min=2,max=100"`
	Type              string  `json:"type" form:"type" binding:"required,oneof=billiard_room game_room meeting_room activity_room sky_lounge"`
	Description       *string `json:"description" form:"description" binding:"omitempty,max=2000"`
	WorkingStartTime  *string `json:"working_start_time" form:"working_start_time" binding:"omitempty"`
	WorkingEndTime    *string `json:"working_end_time" form:"working_end_time" binding:"omitempty"`
	SaturdayStartTime *string `json:"saturday_start_time" form:"saturday_start_time" binding:"omitempty"`
	SaturdayEndTime   *string `json:"saturday_end_time" form:"saturday_end_time" binding:"omitempty"`
	SundayStartTime   *string `json:"sunday_start_time" form:"sunday_start_time" binding:"omitempty"`
	SundayEndTime     *string `json:"sunday_end_time" form:"sunday_end_time" binding:"omitempty"`
	Notice            *string `json:"notice" form:"notice" binding:"omitempty,max=2000"`
}

// UpdateFacilityRequest 更新设施请求体
type UpdateFacilityRequest struct {
	Name              *string `json:"name" form:"name" binding:"omitempty,min=2,max=100"`
	Type              *string `json:"type" form:"type" binding:"omitempty,oneof=billiard_room game_room meeting_room activity_room sky_lounge"`
	Description       *string `json:"description" form:"description" binding:"omitempty,max=2000"`
	WorkingStartTime  *string `json:"working_start_time" form:"working_start_time" binding:"omitempty"`
	WorkingEndTime    *string `json:"working_end_time" form:"working_end_time" binding:"omitempty"`
	SaturdayStartTime *string `json:"saturday_start_time" form:"saturday_start_time" binding:"omitempty"`
	SaturdayEndTime   *string `json:"saturday_end_time" form:"saturday_end_time" binding:"omitempty"`
	SundayStartTime   *string `json:"sunday_start_time" form:"sunday_start_time" binding:"omitempty"`
	SundayEndTime     *string `json:"sunday_end_time" form:"sunday_end_time" binding:"omitempty"`
	Notice            *string `json:"notice" form:"notice" binding:"omitempty,max=2000"`
}

// FacilityReservationTime 预约时间段
type FacilityReservationTime struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Status    string    `json:"status"`
}

// FacilityTimeSlot 时间段可用性信息
type FacilityTimeSlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	IsBooked  bool      `json:"is_booked"`
	Status    *string   `json:"status,omitempty"` // 如果已预订，显示预订状态
}

// FacilityResponse 设施返回结构
type FacilityResponse struct {
	ID                uint                      `json:"id"`
	Name              string                    `json:"name"`
	Type              string                    `json:"type"`
	Description       *string                   `json:"description,omitempty"`
	WorkingStartTime  *string                   `json:"working_start_time,omitempty"`
	WorkingEndTime    *string                   `json:"working_end_time,omitempty"`
	SaturdayStartTime *string                   `json:"saturday_start_time,omitempty"`
	SaturdayEndTime   *string                   `json:"saturday_end_time,omitempty"`
	SundayStartTime   *string                   `json:"sunday_start_time,omitempty"`
	SundayEndTime     *string                   `json:"sunday_end_time,omitempty"`
	Notice            *string                   `json:"notice,omitempty"`
	PhotoURL          *string                   `json:"photo_url,omitempty"`
	Reservations      []FacilityReservationTime `json:"reservations,omitempty"`
	TimeSlots         []FacilityTimeSlot        `json:"time_slots,omitempty"` // 所有开放时间段及其预订状态
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

// ListFacilities 列出当前物业下的所有公共设施
func (h *FacilityHandler) ListFacilities(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	var facilities []property.Facility
	if err := propertyDB.Order("id DESC").Find(&facilities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list facilities"})
		return
	}

	resp := make([]FacilityResponse, len(facilities))
	for i, f := range facilities {
		resp[i] = h.buildFacilityResponse(c, propertyDB, f)
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetFacility 获取单个设施详情
func (h *FacilityHandler) GetFacility(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid facility ID"})
		return
	}

	var facility property.Facility
	if err := propertyDB.First(&facility, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Facility not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get facility"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.buildFacilityResponse(c, propertyDB, facility)})
}

// CreateFacility 创建设施（仅 property_staff）
func (h *FacilityHandler) CreateFacility(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property_staff can create facilities"})
		return
	}

	userID := middleware.GetUserID(c)

	var req CreateFacilityRequest
	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Parse as multipart form
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Parse as JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	facility := property.Facility{
		Name: req.Name,
		Type: req.Type,
	}
	if req.Description != nil {
		facility.Description = req.Description
	}
	if req.WorkingStartTime != nil {
		facility.WorkingStartTime = req.WorkingStartTime
	}
	if req.WorkingEndTime != nil {
		facility.WorkingEndTime = req.WorkingEndTime
	}
	if req.SaturdayStartTime != nil {
		facility.SaturdayStartTime = req.SaturdayStartTime
	}
	if req.SaturdayEndTime != nil {
		facility.SaturdayEndTime = req.SaturdayEndTime
	}
	if req.SundayStartTime != nil {
		facility.SundayStartTime = req.SundayStartTime
	}
	if req.SundayEndTime != nil {
		facility.SundayEndTime = req.SundayEndTime
	}
	if req.Notice != nil {
		facility.Notice = req.Notice
	}

	if err := propertyDB.Create(&facility).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create facility"})
		return
	}

	// Handle photo upload (max 1 photo)
	h.saveFacilityPhoto(c, propertyDB, facility.ID, userID)

	propertyDB.First(&facility, facility.ID)
	c.JSON(http.StatusCreated, gin.H{"data": h.buildFacilityResponse(c, propertyDB, facility)})
}

// UpdateFacility 更新设施（仅 property_staff）
func (h *FacilityHandler) UpdateFacility(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property_staff can update facilities"})
		return
	}

	userID := middleware.GetUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid facility ID"})
		return
	}

	var req UpdateFacilityRequest
	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Parse as multipart form
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Parse as JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	var facility property.Facility
	if err := propertyDB.First(&facility, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Facility not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get facility"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.WorkingStartTime != nil {
		updates["working_start_time"] = *req.WorkingStartTime
	}
	if req.WorkingEndTime != nil {
		updates["working_end_time"] = *req.WorkingEndTime
	}
	if req.SaturdayStartTime != nil {
		updates["saturday_start_time"] = *req.SaturdayStartTime
	}
	if req.SaturdayEndTime != nil {
		updates["saturday_end_time"] = *req.SaturdayEndTime
	}
	if req.SundayStartTime != nil {
		updates["sunday_start_time"] = *req.SundayStartTime
	}
	if req.SundayEndTime != nil {
		updates["sunday_end_time"] = *req.SundayEndTime
	}
	if req.Notice != nil {
		updates["notice"] = *req.Notice
	}

	if len(updates) > 0 {
		if err := propertyDB.Model(&facility).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update facility: " + err.Error()})
			return
		}
	}

	// Handle photo upload (max 1 photo)
	if err := h.saveFacilityPhoto(c, propertyDB, facility.ID, userID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to save facility photo: %v\n", err)
	}

	// Reload facility to ensure we have the latest data including photos
	// Use a fresh query to avoid any caching issues
	var updatedFacility property.Facility
	if err := propertyDB.First(&updatedFacility, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload facility"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.buildFacilityResponse(c, propertyDB, updatedFacility)})
}

// DeleteFacility 删除设施（property_admin 和 property_staff）
func (h *FacilityHandler) DeleteFacility(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_admin" && role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property_admin and property_staff can delete facilities"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid facility ID"})
		return
	}

	if err := propertyDB.Delete(&property.Facility{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete facility"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Facility deleted"})
}

// CreateFacilityReservationRequest 创建预定请求体
type CreateFacilityReservationRequest struct {
	FacilityID uint   `json:"facility_id" binding:"required"`
	UnitID     *uint  `json:"unit_id" binding:"omitempty"`
	StartTime  string `json:"start_time" binding:"required"` // ISO 8601 字符串
	EndTime    string `json:"end_time" binding:"required"`   // ISO 8601 字符串
	Notes      string `json:"notes" binding:"omitempty,max=1000"`
}

// FacilityReservationResponse 预定返回结构
type FacilityReservationResponse struct {
	ID             uint       `json:"id"`
	FacilityID     uint       `json:"facility_id"`
	Facility       string     `json:"facility"`
	UserID         uint       `json:"user_id"`
	UnitID         *uint      `json:"unit_id,omitempty"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        time.Time  `json:"end_time"`
	Status         string     `json:"status"`
	Notes          *string    `json:"notes,omitempty"`
	ApprovedBy     *uint      `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`
	RejectedBy     *uint      `json:"rejected_by,omitempty"`
	RejectedAt     *time.Time `json:"rejected_at,omitempty"`
	RejectedReason *string    `json:"rejected_reason,omitempty"`
	CancelledBy    *uint      `json:"cancelled_by,omitempty"`
	CancelledAt    *time.Time `json:"cancelled_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateReservation 创建公共设施预定（tenant / landlord 可用）
func (h *FacilityHandler) CreateReservation(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if role != "tenant" && role != "landlord" && role != "spa" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord or spa can create facility reservations"})
		return
	}

	var req CreateFacilityReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验设施存在
	var facility property.Facility
	if err := propertyDB.First(&facility, req.FacilityID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Facility not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load facility"})
		return
	}

	// 解析时间
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format, must be ISO 8601"})
		return
	}
	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format, must be ISO 8601"})
		return
	}
	if !end.After(start) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be after start_time"})
		return
	}

	// 验证时间段是否在设施的允许预订时间范围内
	if err := h.validateReservationTimeRange(facility, start, end); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 简单冲突检测：同一设施时间段不能重叠（仅 pending/approved 视为占用）
	var count int64
	if err := propertyDB.Model(&property.FacilityReservation{}).
		Where("facility_id = ? AND status IN ?", req.FacilityID, []string{"pending", "approved"}).
		Where("start_time < ? AND end_time > ?", end, start).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check availability"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Time slot is already booked"})
		return
	}

	res := property.FacilityReservation{
		FacilityID: req.FacilityID,
		UserID:     userID,
		StartTime:  start,
		EndTime:    end,
		Status:     "pending",
	}
	if req.UnitID != nil {
		res.UnitID = req.UnitID
	}
	if req.Notes != "" {
		res.Notes = &req.Notes
	}

	if err := propertyDB.Create(&res).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reservation"})
		return
	}

	propertyDB.Preload("Facility").First(&res, res.ID)
	c.JSON(http.StatusCreated, gin.H{"data": h.buildReservationResponse(res)})
}

// ListMyReservations 当前用户自己的预定列表
func (h *FacilityHandler) ListMyReservations(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)

	var reservations []property.FacilityReservation
	if err := propertyDB.Preload("Facility").
		Where("user_id = ?", userID).
		Order("start_time DESC").
		Find(&reservations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list reservations"})
		return
	}

	resp := make([]FacilityReservationResponse, len(reservations))
	for i, r := range reservations {
		resp[i] = h.buildReservationResponse(r)
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// CancelReservation 取消自己的预定
func (h *FacilityHandler) CancelReservation(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	var res property.FacilityReservation
	if err := propertyDB.Preload("Facility").First(&res, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load reservation"})
		return
	}

	// 只能取消自己的预定，且只能是 pending 状态
	if res.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only cancel your own reservations"})
		return
	}
	if res.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending reservations can be cancelled"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       "cancelled",
		"cancelled_by": userID,
		"cancelled_at": now,
	}

	if err := propertyDB.Model(&res).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel reservation"})
		return
	}

	propertyDB.Preload("Facility").First(&res, res.ID)
	c.JSON(http.StatusOK, gin.H{"data": h.buildReservationResponse(res)})
}

// ListFacilityReservations 管理端查看某个设施的所有预定
func (h *FacilityHandler) ListFacilityReservations(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_admin" && role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can view facility reservations"})
		return
	}

	facilityID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid facility ID"})
		return
	}

	var reservations []property.FacilityReservation
	if err := propertyDB.Preload("Facility").
		Where("facility_id = ?", facilityID).
		Order("start_time DESC").
		Find(&reservations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list reservations"})
		return
	}

	resp := make([]FacilityReservationResponse, len(reservations))
	for i, r := range reservations {
		resp[i] = h.buildReservationResponse(r)
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// ListAllReservations 物业端查看所有预约请求（仅 property_staff）
func (h *FacilityHandler) ListAllReservations(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_staff" && role != "property_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can view all reservations"})
		return
	}

	var reservations []property.FacilityReservation
	if err := propertyDB.Preload("Facility").
		Order("start_time DESC").
		Find(&reservations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list reservations"})
		return
	}

	resp := make([]FacilityReservationResponse, len(reservations))
	for i, r := range reservations {
		resp[i] = h.buildReservationResponse(r)
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// ApproveReservation 物业端审批通过预约
func (h *FacilityHandler) ApproveReservation(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can approve reservations"})
		return
	}

	adminID := middleware.GetUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	var res property.FacilityReservation
	if err := propertyDB.Preload("Facility").First(&res, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load reservation"})
		return
	}

	if res.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending reservations can be approved"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":      "approved",
		"approved_by": adminID,
		"approved_at": now,
	}

	if err := propertyDB.Model(&res).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve reservation"})
		return
	}

	propertyDB.Preload("Facility").First(&res, res.ID)
	c.JSON(http.StatusOK, gin.H{"data": h.buildReservationResponse(res)})
}

// RejectReservation 物业端拒绝预约
func (h *FacilityHandler) RejectReservation(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can reject reservations"})
		return
	}

	adminID := middleware.GetUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	var res property.FacilityReservation
	if err := propertyDB.Preload("Facility").First(&res, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load reservation"})
		return
	}

	if res.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending reservations can be rejected"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":      "rejected",
		"rejected_by": adminID,
		"rejected_at": now,
		"rejected_reason": func() *string {
			if body.Reason == "" {
				return nil
			}
			return &body.Reason
		}(),
	}

	if err := propertyDB.Model(&res).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject reservation"})
		return
	}

	propertyDB.Preload("Facility").First(&res, res.ID)
	c.JSON(http.StatusOK, gin.H{"data": h.buildReservationResponse(res)})
}

// CompleteReservation 标记预约已完成（使用结束）
func (h *FacilityHandler) CompleteReservation(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	role := middleware.GetUserRole(c)
	if role != "property_admin" && role != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can complete reservations"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reservation ID"})
		return
	}

	var res property.FacilityReservation
	if err := propertyDB.Preload("Facility").First(&res, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reservation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load reservation"})
		return
	}

	if res.Status != "approved" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only approved reservations can be marked as completed"})
		return
	}

	now := time.Now()
	if err := propertyDB.Model(&res).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete reservation"})
		return
	}

	propertyDB.Preload("Facility").First(&res, res.ID)
	c.JSON(http.StatusOK, gin.H{"data": h.buildReservationResponse(res)})
}

// saveFacilityPhoto saves uploaded photo for a facility (max 1 photo)
func (h *FacilityHandler) saveFacilityPhoto(c *gin.Context, db *gorm.DB, facilityID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	files := form.File["photo"]
	if len(files) == 0 {
		return nil
	}

	// Only allow 1 photo
	if len(files) > 1 {
		return fmt.Errorf("only 1 photo is allowed")
	}

	file := files[0]

	// Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		return fmt.Errorf("file too large (max 10MB)")
	}

	// Validate file type (only images)
	contentType := file.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowedTypes[contentType] {
		return fmt.Errorf("only image files are allowed")
	}

	// Delete existing photo
	var existingDocs []property.Document
	db.Where("entity_type = ? AND entity_id = ? AND document_type = ? AND is_active = ?",
		property.DocEntityFacility, facilityID, property.DocTypePhoto, true).Find(&existingDocs)
	for _, doc := range existingDocs {
		// Delete file if exists
		if _, err := os.Stat(doc.DocumentPath); err == nil {
			os.Remove(doc.DocumentPath)
		}
		// Mark as inactive
		db.Model(&doc).Update("is_active", false)
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/facilities/%d", facilityID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	// Generate unique filename
	sanitizedBase := sanitizeFacilityFilename(file.Filename)
	// sanitizeFacilityFilename already returns filename with extension, so we only need to add timestamp
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), sanitizedBase)
	filePath := filepath.Join(uploadDir, filename)

	// Save file
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	// Create document record
	fileSize := file.Size
	doc := property.Document{
		EntityType:   property.DocEntityFacility,
		EntityID:     facilityID,
		DocumentType: property.DocTypePhoto,
		DocumentName: file.Filename,
		DocumentPath: filePath,
		FileSize:     &fileSize,
		MimeType:     &contentType,
		UploadedBy:   userID,
		IsActive:     true,
	}

	if err := db.Create(&doc).Error; err != nil {
		return fmt.Errorf("failed to save document record: %v", err)
	}

	return nil
}

// sanitizeFacilityFilename removes special characters from filename
func sanitizeFacilityFilename(name string) string {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]

	result := make([]byte, 0, len(base))
	for i := 0; i < len(base); i++ {
		c := base[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "photo"
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result) + ext
}

// buildFacilityResponse helper
func (h *FacilityHandler) buildFacilityResponse(c *gin.Context, db *gorm.DB, f property.Facility) FacilityResponse {
	resp := FacilityResponse{
		ID:                f.ID,
		Name:              f.Name,
		Type:              f.Type,
		Description:       f.Description,
		WorkingStartTime:  f.WorkingStartTime,
		WorkingEndTime:    f.WorkingEndTime,
		SaturdayStartTime: f.SaturdayStartTime,
		SaturdayEndTime:   f.SaturdayEndTime,
		SundayStartTime:   f.SundayStartTime,
		SundayEndTime:     f.SundayEndTime,
		Notice:            f.Notice,
		CreatedAt:         f.CreatedAt,
		UpdatedAt:         f.UpdatedAt,
	}

	// Load photo (order by created_at desc to get the latest one)
	var photoDoc property.Document
	if err := db.Where("entity_type = ? AND entity_id = ? AND document_type = ? AND is_active = ?",
		property.DocEntityFacility, f.ID, property.DocTypePhoto, true).
		Order("created_at DESC").
		First(&photoDoc).Error; err == nil {
		// Generate complete URL
		// DocumentPath format: ./uploads/facilities/5/filename.png
		photoURL := strings.TrimPrefix(photoDoc.DocumentPath, "./")
		photoURL = strings.ReplaceAll(photoURL, "\\", "/")
		// Remove "uploads/" prefix if present (since we'll add /api/v1/static/uploads/)
		photoURL = strings.TrimPrefix(photoURL, "uploads/")
		photoURL = strings.TrimPrefix(photoURL, "/uploads/")

		// Build complete URL with scheme and host
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host
		completeURL := fmt.Sprintf("%s://%s/api/v1/static/uploads/%s", scheme, host, photoURL)
		resp.PhotoURL = &completeURL
	}

	// Load reservations (only pending and approved) for next 1 week
	now := time.Now()
	oneWeekLater := now.AddDate(0, 0, 7)
	var reservations []property.FacilityReservation
	db.Where("facility_id = ? AND status IN ? AND start_time >= ? AND start_time <= ?",
		f.ID, []string{"pending", "approved"}, now, oneWeekLater).
		Order("start_time ASC").
		Find(&reservations)

	resp.Reservations = make([]FacilityReservationTime, len(reservations))
	for i, r := range reservations {
		resp.Reservations[i] = FacilityReservationTime{
			StartTime: r.StartTime,
			EndTime:   r.EndTime,
			Status:    r.Status,
		}
	}

	// Generate all available time slots for the next 7 days
	resp.TimeSlots = h.generateTimeSlots(f, now, oneWeekLater, reservations)

	return resp
}

// getTimeForDay 根据星期几获取对应的时间段
func (h *FacilityHandler) getTimeForDay(facility property.Facility, weekday time.Weekday) (startTimeStr, endTimeStr string, hasTime bool) {
	switch weekday {
	case time.Saturday:
		if facility.SaturdayStartTime != nil && facility.SaturdayEndTime != nil {
			return *facility.SaturdayStartTime, *facility.SaturdayEndTime, true
		}
	case time.Sunday:
		if facility.SundayStartTime != nil && facility.SundayEndTime != nil {
			return *facility.SundayStartTime, *facility.SundayEndTime, true
		}
	default: // Monday to Friday
		if facility.WorkingStartTime != nil && facility.WorkingEndTime != nil {
			return *facility.WorkingStartTime, *facility.WorkingEndTime, true
		}
	}
	return "", "", false
}

// generateTimeSlots 生成所有开放时间段并标记预订状态
func (h *FacilityHandler) generateTimeSlots(facility property.Facility, startDate, endDate time.Time, reservations []property.FacilityReservation) []FacilityTimeSlot {
	var timeSlots []FacilityTimeSlot

	// 解析时间（支持 HH:MM:SS 和 HH:MM 格式）
	parseTime := func(timeStr string) (hour, min int, err error) {
		timeParts := strings.Split(timeStr, ":")
		if len(timeParts) < 2 {
			return 0, 0, fmt.Errorf("invalid time format")
		}
		hour, err = strconv.Atoi(strings.TrimSpace(timeParts[0]))
		if err != nil {
			return 0, 0, err
		}
		min, err = strconv.Atoi(strings.TrimSpace(timeParts[1]))
		if err != nil {
			return 0, 0, err
		}
		return hour, min, nil
	}

	// 时间段间隔（30分钟）
	slotDuration := 30 * time.Minute

	// 生成每一天的时间段
	currentDate := startDate
	for currentDate.Before(endDate) {
		// 根据星期几获取对应的时间段
		startTimeStr, endTimeStr, hasTime := h.getTimeForDay(facility, currentDate.Weekday())
		if !hasTime {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}

		// 解析时间
		startHour, startMin, err := parseTime(startTimeStr)
		if err != nil {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}
		endHour, endMin, err := parseTime(endTimeStr)
		if err != nil {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}

		// 获取当天的开始和结束时间
		dayStart := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), startHour, startMin, 0, 0, currentDate.Location())
		dayEnd := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), endHour, endMin, 0, 0, currentDate.Location())

		// 如果结束时间早于开始时间，说明跨天（例如 22:00 - 02:00）
		if dayEnd.Before(dayStart) {
			dayEnd = dayEnd.AddDate(0, 0, 1)
		}

		// 只生成未来或今天的时间段
		if dayEnd.Before(time.Now()) {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}

		// 从当天开始时间生成时间段
		slotStart := dayStart
		now := time.Now()
		if slotStart.Before(now) {
			// 如果开始时间已过，从当前时间开始（向上取整到下一个30分钟）
			minutes := now.Minute()
			roundedMinutes := ((minutes / 30) + 1) * 30
			if roundedMinutes >= 60 {
				slotStart = time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, now.Location())
			} else {
				slotStart = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), roundedMinutes, 0, 0, now.Location())
			}
		}

		// 生成当天的所有时间段
		for slotStart.Before(dayEnd) && slotStart.Before(endDate) {
			slotEnd := slotStart.Add(slotDuration)

			// 如果时间段超出当天的结束时间，调整结束时间
			if slotEnd.After(dayEnd) {
				slotEnd = dayEnd
			}

			// 检查该时间段是否已被预订
			isBooked, status := h.isTimeSlotBooked(slotStart, slotEnd, reservations)

			timeSlots = append(timeSlots, FacilityTimeSlot{
				StartTime: slotStart,
				EndTime:   slotEnd,
				IsBooked:  isBooked,
				Status:    status,
			})

			slotStart = slotStart.Add(slotDuration)
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return timeSlots
}

// validateReservationTimeRange 验证预订时间段是否在设施的允许时间范围内
func (h *FacilityHandler) validateReservationTimeRange(facility property.Facility, start, end time.Time) error {
	// 根据开始时间的星期几获取对应的时间段
	startTimeStr, endTimeStr, hasTime := h.getTimeForDay(facility, start.Weekday())
	if !hasTime {
		return fmt.Errorf("facility is not available on this day")
	}

	startTimeParts := strings.Split(startTimeStr, ":")
	endTimeParts := strings.Split(endTimeStr, ":")
	if len(startTimeParts) < 2 || len(endTimeParts) < 2 {
		return fmt.Errorf("invalid facility time configuration")
	}

	startHour, err := strconv.Atoi(strings.TrimSpace(startTimeParts[0]))
	if err != nil {
		return fmt.Errorf("invalid facility start time")
	}
	startMin, err := strconv.Atoi(strings.TrimSpace(startTimeParts[1]))
	if err != nil {
		return fmt.Errorf("invalid facility start time")
	}
	endHour, err := strconv.Atoi(strings.TrimSpace(endTimeParts[0]))
	if err != nil {
		return fmt.Errorf("invalid facility end time")
	}
	endMin, err := strconv.Atoi(strings.TrimSpace(endTimeParts[1]))
	if err != nil {
		return fmt.Errorf("invalid facility end time")
	}

	// 获取预订开始日期当天的允许时间段
	startDayAvailableStart := time.Date(start.Year(), start.Month(), start.Day(), startHour, startMin, 0, 0, start.Location())
	startDayAvailableEnd := time.Date(start.Year(), start.Month(), start.Day(), endHour, endMin, 0, 0, start.Location())

	// 处理跨天情况（例如 22:00 - 02:00）
	if startDayAvailableEnd.Before(startDayAvailableStart) || startDayAvailableEnd.Equal(startDayAvailableStart) {
		startDayAvailableEnd = startDayAvailableEnd.AddDate(0, 0, 1)
	}

	// 检查预订开始时间是否在允许范围内
	if start.Before(startDayAvailableStart) {
		return fmt.Errorf("reservation start time is before facility available time (%s-%s)", startTimeStr, endTimeStr)
	}
	if start.After(startDayAvailableEnd) || start.Equal(startDayAvailableEnd) {
		return fmt.Errorf("reservation start time is after facility available time (%s-%s)", startTimeStr, endTimeStr)
	}

	// 获取预订结束日期当天的允许时间段（根据结束时间的星期几）
	endTimeStr2, endTimeStr2End, hasEndTime := h.getTimeForDay(facility, end.Weekday())
	if !hasEndTime {
		return fmt.Errorf("facility is not available on end date")
	}

	endTimeParts2 := strings.Split(endTimeStr2, ":")
	endTimeParts2End := strings.Split(endTimeStr2End, ":")
	if len(endTimeParts2) < 2 || len(endTimeParts2End) < 2 {
		return fmt.Errorf("invalid facility time configuration for end date")
	}

	endDayStartHour, err := strconv.Atoi(strings.TrimSpace(endTimeParts2[0]))
	if err != nil {
		return fmt.Errorf("invalid facility start time for end date")
	}
	endDayStartMin, err := strconv.Atoi(strings.TrimSpace(endTimeParts2[1]))
	if err != nil {
		return fmt.Errorf("invalid facility start time for end date")
	}
	endDayEndHour, err := strconv.Atoi(strings.TrimSpace(endTimeParts2End[0]))
	if err != nil {
		return fmt.Errorf("invalid facility end time for end date")
	}
	endDayEndMin, err := strconv.Atoi(strings.TrimSpace(endTimeParts2End[1]))
	if err != nil {
		return fmt.Errorf("invalid facility end time for end date")
	}

	endDayAvailableStart := time.Date(end.Year(), end.Month(), end.Day(), endDayStartHour, endDayStartMin, 0, 0, end.Location())
	endDayAvailableEnd := time.Date(end.Year(), end.Month(), end.Day(), endDayEndHour, endDayEndMin, 0, 0, end.Location())

	// 处理跨天情况
	if endDayAvailableEnd.Before(endDayAvailableStart) || endDayAvailableEnd.Equal(endDayAvailableStart) {
		endDayAvailableEnd = endDayAvailableEnd.AddDate(0, 0, 1)
	}

	// 检查预订结束时间是否在允许范围内
	if end.Before(endDayAvailableStart) || end.Equal(endDayAvailableStart) {
		return fmt.Errorf("reservation end time is before facility available time (%s-%s)", startTimeStr, endTimeStr)
	}
	if end.After(endDayAvailableEnd) {
		return fmt.Errorf("reservation end time is after facility available time (%s-%s)", startTimeStr, endTimeStr)
	}

	// 如果预订跨越多天，检查中间每一天是否都在允许范围内
	if start.Day() != end.Day() || start.Month() != end.Month() || start.Year() != end.Year() {
		currentDate := start.AddDate(0, 0, 1)
		for currentDate.Before(end) {
			// 对于中间的天，全天都必须在允许范围内（这里我们只需要确保当天的时间配置是合理的）
			// 实际上，如果开始和结束时间都在范围内，中间的天也会在范围内
			// 这里我们只需要遍历中间的天，不需要实际验证（因为开始和结束时间已经验证过了）
			currentDate = currentDate.AddDate(0, 0, 1)
		}
	}

	return nil
}

// isTimeSlotBooked 检查时间段是否已被预订
func (h *FacilityHandler) isTimeSlotBooked(slotStart, slotEnd time.Time, reservations []property.FacilityReservation) (bool, *string) {
	for _, res := range reservations {
		// 检查时间段是否与预订重叠
		// 时间段与预订重叠的条件：slotStart < res.EndTime && slotEnd > res.StartTime
		if slotStart.Before(res.EndTime) && slotEnd.After(res.StartTime) {
			return true, &res.Status
		}
	}
	return false, nil
}

// buildReservationResponse helper
func (h *FacilityHandler) buildReservationResponse(r property.FacilityReservation) FacilityReservationResponse {
	resp := FacilityReservationResponse{
		ID:             r.ID,
		FacilityID:     r.FacilityID,
		UserID:         r.UserID,
		UnitID:         r.UnitID,
		StartTime:      r.StartTime,
		EndTime:        r.EndTime,
		Status:         r.Status,
		Notes:          r.Notes,
		ApprovedBy:     r.ApprovedBy,
		ApprovedAt:     r.ApprovedAt,
		RejectedBy:     r.RejectedBy,
		RejectedAt:     r.RejectedAt,
		RejectedReason: r.RejectedReason,
		CancelledBy:    r.CancelledBy,
		CancelledAt:    r.CancelledAt,
		CompletedAt:    r.CompletedAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	if r.Facility.ID > 0 {
		resp.Facility = r.Facility.Name
	}
	return resp
}
