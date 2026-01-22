package handler

import (
	"encoding/json"
	"homexai/internal/middleware"
	"homexai/internal/models/property"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SettingsHandler struct {
	masterDB *gorm.DB
}

func NewSettingsHandler(masterDB *gorm.DB) *SettingsHandler {
	return &SettingsHandler{
		masterDB: masterDB,
	}
}

// PaymentMethodsSettings represents payment methods configuration
type PaymentMethodsSettings struct {
	GCash struct {
		Enabled     bool   `json:"enabled"`
		Number      string `json:"number"`
		AccountName string `json:"account_name"`
	} `json:"gcash"`
	Maya struct {
		Enabled     bool   `json:"enabled"`
		Number      string `json:"number"`
		AccountName string `json:"account_name"`
	} `json:"maya"`
	Card struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
	} `json:"card"`
	Bank struct {
		Enabled  bool `json:"enabled"`
		Accounts []struct {
			BankName      string `json:"bank_name"`
			AccountNumber string `json:"account_number"`
			AccountName   string `json:"account_name"`
		} `json:"accounts"`
	} `json:"bank"`
	CustomInstructions string `json:"custom_instructions"`
}

// GetPaymentMethods gets payment methods settings
// @Summary Get payment methods settings
// @Description Get payment methods configuration for the property
// @Tags Settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /settings/payment-methods [get]
func (h *SettingsHandler) GetPaymentMethods(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	// Default settings
	defaultSettings := PaymentMethodsSettings{
		GCash: struct {
			Enabled     bool   `json:"enabled"`
			Number      string `json:"number"`
			AccountName string `json:"account_name"`
		}{
			Enabled:     false,
			Number:      "",
			AccountName: "",
		},
		Maya: struct {
			Enabled     bool   `json:"enabled"`
			Number      string `json:"number"`
			AccountName string `json:"account_name"`
		}{
			Enabled:     false,
			Number:      "",
			AccountName: "",
		},
		Card: struct {
			Enabled  bool   `json:"enabled"`
			Provider string `json:"provider"`
		}{
			Enabled:  false,
			Provider: "",
		},
		Bank: struct {
			Enabled  bool `json:"enabled"`
			Accounts []struct {
				BankName      string `json:"bank_name"`
				AccountNumber string `json:"account_number"`
				AccountName   string `json:"account_name"`
			} `json:"accounts"`
		}{
			Enabled:  false,
			Accounts: []struct {
				BankName      string `json:"bank_name"`
				AccountNumber string `json:"account_number"`
				AccountName   string `json:"account_name"`
			}{},
		},
		CustomInstructions: "",
	}

	// Try to load from property_settings table
	var setting property.PropertySettings
	err := propertyDB.Where("setting_key = ?", property.SettingKeyPaymentMethods).First(&setting).Error
	if err == nil && setting.SettingValue != "" {
		// Parse JSON settings
		if err := json.Unmarshal([]byte(setting.SettingValue), &defaultSettings); err == nil {
			utils.SuccessResponse(c, "Payment methods settings retrieved", defaultSettings)
			return
		}
	}

	// Return defaults if not found or parse error
	utils.SuccessResponse(c, "Payment methods settings retrieved", defaultSettings)
}

// UpdatePaymentMethods updates payment methods settings
// @Summary Update payment methods settings
// @Description Update payment methods configuration for the property
// @Tags Settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param settings body PaymentMethodsSettings true "Payment methods settings"
// @Success 200 {object} map[string]interface{}
// @Router /settings/payment-methods [post]
func (h *SettingsHandler) UpdatePaymentMethods(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	var settings PaymentMethodsSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Serialize settings to JSON
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to serialize settings", err)
		return
	}

	// Save or update in property_settings table
	var setting property.PropertySettings
	err = propertyDB.Where("setting_key = ?", property.SettingKeyPaymentMethods).First(&setting).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new setting
		setting = property.PropertySettings{
			SettingKey:   property.SettingKeyPaymentMethods,
			SettingValue: string(settingsJSON),
		}
		if err := propertyDB.Create(&setting).Error; err != nil {
			utils.InternalServerErrorResponse(c, "Failed to save settings", err)
			return
		}
	} else if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to query settings", err)
		return
	} else {
		// Update existing setting
		setting.SettingValue = string(settingsJSON)
		if err := propertyDB.Save(&setting).Error; err != nil {
			utils.InternalServerErrorResponse(c, "Failed to update settings", err)
			return
		}
	}

	utils.SuccessResponse(c, "Payment methods settings saved successfully", settings)
}

