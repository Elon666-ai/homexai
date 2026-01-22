package master

import (
	"time"
)

// Translation 多语言翻译表
type Translation struct {
	ID               uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	TranslationKey   string    `gorm:"column:translation_key;type:varchar(200);not null;uniqueIndex:uk_key_language,priority:1;index:idx_translation_key" json:"translation_key" default:"" comment:"翻译键"`
	Language         string    `gorm:"column:language;type:varchar(10);not null;uniqueIndex:uk_key_language,priority:2;index:idx_language" json:"language" default:"" comment:"语言代码(en,zh-CN,zh-TW,tl)"`
	TranslationValue string    `gorm:"column:translation_value;type:text;not null" json:"translation_value" default:"" comment:"翻译内容"`
	Context          *string   `gorm:"column:context;type:varchar(100);index:idx_context" json:"context" default:"null" comment:"上下文(role,permission等)"`
	CreatedAt        time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
}

func (Translation) TableName() string {
	return "translations"
}

// Language constants
const (
	LangEnglish            = "en"
	LangChineseSimplified  = "zh-CN"
	LangChineseTraditional = "zh-TW"
	LangTagalog            = "tl"
)

// IsValidLanguage 检查语言代码是否有效
func IsValidLanguage(lang string) bool {
	validLanguages := []string{LangEnglish, LangChineseSimplified, LangChineseTraditional, LangTagalog}
	for _, validLang := range validLanguages {
		if lang == validLang {
			return true
		}
	}
	return false
}
