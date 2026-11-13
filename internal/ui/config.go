package ui

import (
	"io/fs"
	"regexp"
	"strconv"

	"golang.org/x/image/font/opentype"
)

type appSettings struct {
	name    string
	version string
}

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?$`)

func loadConfig(config *element, fsys fs.FS, doc *Document) error {
	if config.name != "config" {
		return at(config.source, "config.xml must have a <config> root")
	}
	if err := config.attributes(); err != nil {
		return err
	}
	if err := config.noText(); err != nil {
		return err
	}

	var fonts, app *element
	window := doc.Window
	windowSet := false
	for _, child := range config.children {
		switch child.name {
		case "fonts":
			if fonts != nil {
				return at(child.source, "<config> can contain only one <fonts> element")
			}
			fonts = child
		case "app":
			if app != nil {
				return at(child.source, "<config> can contain only one <app> element")
			}
			app = child
		case "window":
			if windowSet {
				return at(child.source, "<config> can contain only one <window> element")
			}
			windowSet = true
			parsed, err := loadWindow(child, doc.Window)
			if err != nil {
				return err
			}
			window = parsed
		default:
			return at(child.source, "unsupported tag <%s> in <config>; config.xml requires <fonts>", child.name)
		}
	}

	if app != nil {
		settings, err := loadApp(app)
		if err != nil {
			return err
		}
		doc.AppName, doc.Version = settings.name, settings.version
	}
	if windowSet {
		doc.Window = window
	}
	if fonts == nil {
		return at(config.source, "<config> requires exactly one <fonts> element")
	}
	fontsMap, err := loadFonts(fonts, fsys)
	if err != nil {
		return err
	}
	doc.Fonts = fontsMap
	return nil
}

func loadApp(e *element) (appSettings, error) {
	if err := e.attributes("name", "version"); err != nil {
		return appSettings{}, err
	}
	if len(e.children) != 0 {
		return appSettings{}, at(e.source, "<app> cannot contain child elements")
	}
	if err := e.noText(); err != nil {
		return appSettings{}, err
	}
	name, err := e.required("name")
	if err != nil {
		return appSettings{}, err
	}
	version, err := e.required("version")
	if err != nil {
		return appSettings{}, err
	}
	if !versionPattern.MatchString(version) {
		return appSettings{}, at(e.source, "app version must be major.minor.patch or major.minor.patch.build")
	}
	return appSettings{name: name, version: version}, nil
}

func loadFonts(fonts *element, fsys fs.FS) (map[string]*opentype.Font, error) {
	if err := fonts.attributes(); err != nil {
		return nil, err
	}
	if err := fonts.noText(); err != nil {
		return nil, err
	}

	result := make(map[string]*opentype.Font)
	for _, entry := range fonts.children {
		if entry.name != "font" {
			return nil, at(entry.source, "unsupported tag <%s> in <fonts>", entry.name)
		}
		if err := entry.attributes("name", "src"); err != nil {
			return nil, err
		}
		if len(entry.children) != 0 {
			return nil, at(entry.source, "<font> cannot contain child elements")
		}
		if err := entry.noText(); err != nil {
			return nil, err
		}
		name, err := entry.required("name")
		if err != nil {
			return nil, err
		}
		if _, exists := result[name]; exists {
			return nil, at(entry.source, "duplicate font name %q", name)
		}
		src, err := entry.required("src")
		if err != nil {
			return nil, err
		}
		asset, err := assetPath(src)
		if err != nil {
			return nil, at(entry.source, "font src must be relative to the UI directory")
		}
		data, err := fs.ReadFile(fsys, asset)
		if err != nil {
			return nil, at(entry.source, "read font %q: %w", name, err)
		}
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, at(entry.source, "parse font %q (%s): %w", name, src, err)
		}
		result[name] = parsed
	}
	if _, exists := result["default"]; !exists {
		return nil, at(fonts.source, "<fonts> must register a font named %q", "default")
	}
	return result, nil
}

func loadWindow(e *element, defaults WindowConfig) (WindowConfig, error) {
	if err := e.attributes("backend", "display", "width", "height", "fullscreen", "decorated", "resizable", "min-width", "min-height"); err != nil {
		return WindowConfig{}, err
	}
	if len(e.children) != 0 {
		return WindowConfig{}, at(e.source, "<window> cannot contain child elements")
	}
	if err := e.noText(); err != nil {
		return WindowConfig{}, err
	}
	if value, exists := e.attrs["backend"]; exists {
		if value != "desktop" && value != "drm" {
			return WindowConfig{}, at(e.source, "window backend must be desktop or drm")
		}
		defaults.Backend = value
	}
	for _, item := range []struct {
		name   string
		target *int
		min    int
	}{{"display", &defaults.Display, 0}, {"width", &defaults.Width, 1}, {"height", &defaults.Height, 1}} {
		if value, exists := e.attrs[item.name]; exists {
			number, err := strconv.Atoi(value)
			if err != nil || number < item.min {
				return WindowConfig{}, at(e.source, "window %s must be an integer >= %d", item.name, item.min)
			}
			*item.target = number
		}
	}
	if value, exists := e.attrs["fullscreen"]; exists {
		fullscreen, err := boolean(value)
		if err != nil {
			return WindowConfig{}, at(e.source, "window fullscreen must be true or false")
		}
		defaults.Fullscreen = fullscreen
	}
	for _, item := range []struct {
		name   string
		target *bool
	}{{"decorated", &defaults.Decorated}, {"resizable", &defaults.Resizable}} {
		if value, exists := e.attrs[item.name]; exists {
			parsed, err := boolean(value)
			if err != nil {
				return WindowConfig{}, at(e.source, "window %s must be true or false", item.name)
			}
			*item.target = parsed
		}
	}
	for _, item := range []struct {
		name   string
		target *int
	}{{"min-width", &defaults.MinWidth}, {"min-height", &defaults.MinHeight}} {
		if value, exists := e.attrs[item.name]; exists {
			number, err := strconv.Atoi(value)
			if err != nil || number <= 0 {
				return WindowConfig{}, at(e.source, "window %s must be a positive integer", item.name)
			}
			*item.target = number
		}
	}
	if !defaults.Fullscreen && defaults.MinWidth > defaults.Width {
		return WindowConfig{}, at(e.source, "window min-width cannot exceed width")
	}
	if !defaults.Fullscreen && defaults.MinHeight > defaults.Height {
		return WindowConfig{}, at(e.source, "window min-height cannot exceed height")
	}
	return defaults, nil
}
