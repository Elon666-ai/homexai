package master

import (
	"homexai/internal/models/master"
	"time"

	"gorm.io/gorm"
)

type OAuthProviderRepository struct {
	db *gorm.DB
}

func NewOAuthProviderRepository(db *gorm.DB) *OAuthProviderRepository {
	return &OAuthProviderRepository{db: db}
}

// Create creates a new OAuth provider record
func (r *OAuthProviderRepository) Create(provider *master.OAuthProvider) error {
	return r.db.Create(provider).Error
}

// FindByProviderAndUserID finds OAuth provider by provider name and provider user ID
func (r *OAuthProviderRepository) FindByProviderAndUserID(provider, providerUserID string) (*master.OAuthProvider, error) {
	var oauthProvider master.OAuthProvider
	err := r.db.Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&oauthProvider).Error
	if err != nil {
		return nil, err
	}
	return &oauthProvider, nil
}

// FindByUserID finds all OAuth providers for a user
func (r *OAuthProviderRepository) FindByUserID(userID uint) ([]master.OAuthProvider, error) {
	var providers []master.OAuthProvider
	err := r.db.Where("user_id = ?", userID).Find(&providers).Error
	return providers, err
}

// UpdateTokens updates OAuth provider tokens
func (r *OAuthProviderRepository) UpdateTokens(id uint, accessToken, refreshToken string, expiresAt *time.Time) error {
	updates := map[string]interface{}{
		"access_token":     accessToken,
		"token_expires_at": expiresAt,
	}
	if refreshToken != "" {
		updates["refresh_token"] = refreshToken
	}
	return r.db.Model(&master.OAuthProvider{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteByUserIDAndProvider deletes OAuth provider by user ID and provider name
func (r *OAuthProviderRepository) DeleteByUserIDAndProvider(userID uint, provider string) error {
	return r.db.Where("user_id = ? AND provider = ?", userID, provider).Delete(&master.OAuthProvider{}).Error
}

// FindByUserIDAndProvider finds OAuth provider by user ID and provider name
func (r *OAuthProviderRepository) FindByUserIDAndProvider(userID uint, provider string) (*master.OAuthProvider, error) {
	var oauthProvider master.OAuthProvider
	err := r.db.Where("user_id = ? AND provider = ?", userID, provider).First(&oauthProvider).Error
	if err != nil {
		return nil, err
	}
	return &oauthProvider, nil
}
