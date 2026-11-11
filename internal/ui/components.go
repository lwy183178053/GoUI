package ui

import (
	"path/filepath"
	"strings"
)

// expandChildren 在加载前展开组件文件，并给组件里的 ID 加实例名前缀。
func expandChildren(children []*element, directory, namespace string, stack, names map[string]bool, uiDirectory string) ([]*element, error) {
	var result []*element
	for _, child := range children {
		if child.name == "component" {
			expanded, err := expandComponent(child, directory, namespace, stack, names, uiDirectory)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded...)
			continue
		}
		if namespace != "" {
			if id := child.attrs["id"]; id != "" {
				child.attrs["id"] = namespace + "." + strings.TrimSpace(id)
			}
		}
		children, err := expandChildren(child.children, directory, namespace, stack, names, uiDirectory)
		if err != nil {
			return nil, err
		}
		child.children = children
		result = append(result, child)
	}
	return result, nil
}

func expandComponent(reference *element, directory, namespace string, stack, names map[string]bool, uiDirectory string) ([]*element, error) {
	if err := reference.attributes("src", "name"); err != nil {
		return nil, err
	}
	if len(reference.children) != 0 {
		return nil, at(reference.source, "component reference cannot contain child elements")
	}
	if err := reference.noText(); err != nil {
		return nil, err
	}
	src, err := reference.required("src")
	if err != nil {
		return nil, err
	}
	name, err := reference.required("name")
	if err != nil {
		return nil, err
	}
	if strings.Contains(name, ".") {
		return nil, at(reference.source, "component name must not contain a dot")
	}
	instance := name
	if namespace != "" {
		instance = namespace + "." + name
	}
	if names[instance] {
		return nil, at(reference.source, "duplicate component name %q", instance)
	}
	names[instance] = true

	src = filepath.FromSlash(src)
	if filepath.IsAbs(src) || filepath.VolumeName(src) != "" || strings.HasPrefix(src, `\`) {
		return nil, at(reference.source, "component src must be relative to its XML file")
	}
	path, err := filepath.Abs(filepath.Join(directory, src))
	if err != nil {
		return nil, at(reference.source, "resolve component path: %w", err)
	}
	relative, err := filepath.Rel(uiDirectory, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, at(reference.source, "component src must stay inside the UI directory")
	}
	if stack[path] {
		return nil, at(reference.source, "component cycle detected at %q", src)
	}
	component, err := readXML(path)
	if err != nil {
		return nil, err
	}
	if component.name != "component" {
		return nil, at(component.source, "component file must have a <component> root")
	}
	if err := component.attributes(); err != nil {
		return nil, err
	}
	if err := component.noText(); err != nil {
		return nil, err
	}
	stack[path] = true
	children, err := expandChildren(component.children, filepath.Dir(path), instance, stack, names, uiDirectory)
	delete(stack, path)
	return children, err
}
