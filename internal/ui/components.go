package ui

import (
	"io/fs"
	"path"
	"strings"
)

// expandChildren 在加载前展开组件文件，并给组件里的 ID 加实例名前缀。
func expandChildren(fsys fs.FS, children []*element, xmlPath, namespace string, stack, names map[string]bool, sourceRoot string) ([]*element, error) {
	var result []*element
	for _, child := range children {
		if child.name == "component" {
			expanded, err := expandComponent(fsys, child, path.Dir(xmlPath), namespace, stack, names, sourceRoot)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded...)
			continue
		}
		if namespace != "" {
			if id, exists := child.attrs["id"]; exists {
				id = strings.TrimSpace(id)
				if id == "" {
					return nil, at(child.source, "id must not be empty")
				}
				child.attrs["id"] = namespace + "." + id
			}
		}
		children, err := expandChildren(fsys, child.children, xmlPath, namespace, stack, names, sourceRoot)
		if err != nil {
			return nil, err
		}
		child.children = children
		result = append(result, child)
	}
	return result, nil
}

func expandComponent(fsys fs.FS, reference *element, directory, namespace string, stack, names map[string]bool, sourceRoot string) ([]*element, error) {
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

	if strings.Contains(src, `\`) || strings.Contains(src, ":") || path.IsAbs(src) {
		return nil, at(reference.source, "component src must be relative to its XML file")
	}
	componentPath := path.Clean(path.Join(directory, src))
	if componentPath == ".." || strings.HasPrefix(componentPath, "../") || !fs.ValidPath(componentPath) {
		return nil, at(reference.source, "component src must stay inside the UI directory")
	}
	if stack[componentPath] {
		return nil, at(reference.source, "component cycle detected at %q", src)
	}
	component, err := readXML(fsys, componentPath, sourceName(sourceRoot, componentPath))
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
	stack[componentPath] = true
	children, err := expandChildren(fsys, component.children, componentPath, instance, stack, names, sourceRoot)
	delete(stack, componentPath)
	return children, err
}
