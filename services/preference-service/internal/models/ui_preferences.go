package models

type UIPreferences struct {
	Theme            ThemeMode              `json:"theme" bson:"theme"`
	ColorScheme      string                 `json:"colorScheme,omitempty" bson:"colorScheme,omitempty"`
	FontSize         int                    `json:"fontSize" bson:"fontSize"` // Base font size in pixels
	FontFamily       string                 `json:"fontFamily,omitempty" bson:"fontFamily,omitempty"`
	SidebarCollapsed bool                   `json:"sidebarCollapsed" bson:"sidebarCollapsed"`
	EnableAnimations bool                   `json:"enableAnimations" bson:"enableAnimations"`
	DashboardLayout  []string               `json:"dashboardLayout,omitempty" bson:"dashboardLayout,omitempty"` // Widget IDs in display order
	DefaultViews     map[string]string      `json:"defaultViews,omitempty" bson:"defaultViews,omitempty"`       // Section -> view type mapping
	Custom           map[string]interface{} `json:"custom,omitempty" bson:"custom,omitempty"`                   // For custom UI settings
}

// ThemeSettings contains detailed theme configuration
type ThemeSettings struct {
	Name            string            `json:"name" bson:"name"`
	PrimaryColor    string            `json:"primaryColor" bson:"primaryColor"` // Hex color code
	SecondaryColor  string            `json:"secondaryColor" bson:"secondaryColor"`
	BackgroundColor string            `json:"backgroundColor" bson:"backgroundColor"`
	TextColor       string            `json:"textColor" bson:"textColor"`
	AccentColors    map[string]string `json:"accentColors,omitempty" bson:"accentColors,omitempty"`
	Custom          map[string]string `json:"custom,omitempty" bson:"custom,omitempty"`
}

// AccessibilityPreferences represents accessibility settings
type AccessibilityPreferences struct {
	HighContrast             bool                   `json:"highContrast" bson:"highContrast"`
	ReduceMotion             bool                   `json:"reduceMotion" bson:"reduceMotion"`
	ScreenReaderOptimized    bool                   `json:"screenReaderOptimized" bson:"screenReaderOptimized"`
	FontScaling              float64                `json:"fontScaling" bson:"fontScaling"` // 1.0 is normal
	EnableKeyboardNavigation bool                   `json:"enableKeyboardNavigation" bson:"enableKeyboardNavigation"`
	ColorBlindMode           string                 `json:"colorBlindMode,omitempty" bson:"colorBlindMode,omitempty"` // protanopia, deuteranopia, tritanopia
	Custom                   map[string]interface{} `json:"custom,omitempty" bson:"custom,omitempty"`
}
