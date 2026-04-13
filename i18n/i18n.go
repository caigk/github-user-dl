package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Translator 翻译器结构
type Translator struct {
	lang         string
	translations map[string]map[string]string
}

// 全局翻译器实例
var T *Translator

// Init 初始化翻译器，自动检测系统语言
func Init() {
	// 获取系统语言设置
	lang := detectSystemLanguage()
	T = NewTranslator(lang)
}

// NewTranslator 创建指定语言的翻译器
func NewTranslator(lang string) *Translator {
	t := &Translator{
		lang:         lang,
		translations: make(map[string]map[string]string),
	}

	// 注册翻译映射
	t.translations["en"] = En
	t.translations["zh"] = Zh

	return t
}

// detectSystemLanguage 检测系统语言设置
func detectSystemLanguage() string {
	// 优先检查环境变量
	lang := os.Getenv("LANG")
	if lang != "" {
		lang = strings.Split(lang, ".")[0]
		lang = strings.Split(lang, "_")[0]
		if strings.HasPrefix(lang, "zh") {
			return "zh"
		}
		if strings.HasPrefix(lang, "en") {
			return "en"
		}
	}

	// 检查 LC_MESSAGES
	lang = os.Getenv("LC_MESSAGES")
	if lang != "" {
		lang = strings.Split(lang, ".")[0]
		lang = strings.Split(lang, "_")[0]
		if strings.HasPrefix(lang, "zh") {
			return "zh"
		}
		if strings.HasPrefix(lang, "en") {
			return "en"
		}
	}

	// 默认使用英文
	return "en"
}

// SetLanguage 设置语言
func (t *Translator) SetLanguage(lang string) {
	if _, ok := t.translations[lang]; ok {
		t.lang = lang
	}
}

// GetLanguage 获取当前语言
func (t *Translator) GetLanguage() string {
	return t.lang
}

// Tr 翻译文本
func (t *Translator) Tr(key string, args ...interface{}) string {
	translations, ok := t.translations[t.lang]
	if !ok {
		translations = t.translations["en"]
	}

	text, ok := translations[key]
	if !ok {
		// 如果找不到翻译，返回 key 本身
		return key
	}

	if len(args) == 0 {
		return text
	}

	return fmt.Sprintf(text, args...)
}

// TrDefault 如果找不到翻译则返回默认值
func (t *Translator) TrDefault(key string, defaultText string, args ...interface{}) string {
	translations, ok := t.translations[t.lang]
	if !ok {
		translations = t.translations["en"]
	}

	text, ok := translations[key]
	if !ok {
		text = defaultText
	}

	if len(args) == 0 {
		return text
	}

	return fmt.Sprintf(text, args...)
}
