package config

// UIColorConfig UI 颜色配置
type UIColorConfig struct {
	Primary   string `mapstructure:"primary"`
	Secondary string `mapstructure:"secondary"`
	Muted     string `mapstructure:"muted"`
	Error     string `mapstructure:"error"`
	Warning   string `mapstructure:"warning"`
	Success   string `mapstructure:"success"`
}

// GetColors 获取颜色配置（使用默认值）
func (c *UIColorConfig) GetColors() *UIColorConfig {
	if c == nil {
		return &UIColorConfig{
			Primary:   UIColors.Primary,
			Secondary: UIColors.Secondary,
			Muted:     UIColors.Muted,
			Error:     UIColors.Error,
			Warning:   UIColors.Warning,
			Success:   UIColors.Success,
		}
	}

	// 合并默认值
	colors := &UIColorConfig{
		Primary:   c.Primary,
		Secondary: c.Secondary,
		Muted:     c.Muted,
		Error:     c.Error,
		Warning:   c.Warning,
		Success:   c.Success,
	}

	if colors.Primary == "" {
		colors.Primary = UIColors.Primary
	}
	if colors.Secondary == "" {
		colors.Secondary = UIColors.Secondary
	}
	if colors.Muted == "" {
		colors.Muted = UIColors.Muted
	}
	if colors.Error == "" {
		colors.Error = UIColors.Error
	}
	if colors.Warning == "" {
		colors.Warning = UIColors.Warning
	}
	if colors.Success == "" {
		colors.Success = UIColors.Success
	}

	return colors
}

// UIConfig UI 配置
type UIConfig struct {
	Colors  *UIColorConfig `mapstructure:"ui_colors"`
	IconSet string         `mapstructure:"ui_icon_set"` // "emoji" or "ascii"
}

// GetColors 获取 UI 颜色配置
func (c *UIConfig) GetColors() *UIColorConfig {
	return c.Colors.GetColors()
}
