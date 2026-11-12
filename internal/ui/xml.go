package ui

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"strings"
)

// element 暂存 XML 结构，供各标签分别校验。
type element struct {
	name     string
	attrs    map[string]string
	children []*element
	text     strings.Builder
	source   Source
}

func readXML(fsys fs.FS, path, sourceName string) (*element, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, at(Source{sourceName, 1, 1}, "read XML: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *element
	var stack []*element
	for {
		line, column := decoder.InputPos()
		source := Source{sourceName, line, column}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			line, column = decoder.InputPos()
			var syntax *xml.SyntaxError
			if errors.As(err, &syntax) {
				line = syntax.Line
			}
			return nil, at(Source{sourceName, line, column}, "invalid XML: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Space != "" {
				return nil, at(source, "XML namespaces are not supported")
			}
			current := &element{name: token.Name.Local, attrs: make(map[string]string), source: source}
			for _, attr := range token.Attr {
				if attr.Name.Space != "" || attr.Name.Local == "xmlns" {
					return nil, at(source, "XML namespaces are not supported")
				}
				if _, exists := current.attrs[attr.Name.Local]; exists {
					return nil, at(source, "duplicate attribute %q", attr.Name.Local)
				}
				current.attrs[attr.Name.Local] = attr.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, at(source, "XML must contain exactly one root element")
				}
				root = current
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, current)
			}
			stack = append(stack, current)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(token)) != "" {
					return nil, at(source, "text outside the root element is not allowed")
				}
			} else {
				stack[len(stack)-1].text.Write(token)
			}
		case xml.Directive:
			return nil, at(source, "XML directives are not supported")
		case xml.ProcInst:
			if token.Target != "xml" || root != nil {
				return nil, at(source, "unsupported XML processing instruction %q", token.Target)
			}
		}
	}
	if root == nil {
		return nil, at(Source{sourceName, 1, 1}, "XML has no root element")
	}
	if len(stack) != 0 {
		return nil, at(stack[len(stack)-1].source, "unclosed element")
	}
	return root, nil
}

func (e *element) attributes(allowed ...string) error {
	for name := range e.attrs {
		found := false
		for _, candidate := range allowed {
			if name == candidate {
				found = true
				break
			}
		}
		if !found {
			return at(e.source, "unsupported attribute %q on <%s>", name, e.name)
		}
	}
	return nil
}

func (e *element) noText() error {
	if strings.TrimSpace(e.text.String()) != "" {
		return at(e.source, "<%s> cannot contain text", e.name)
	}
	return nil
}

func (e *element) required(name string) (string, error) {
	value, exists := e.attrs[name]
	if !exists || strings.TrimSpace(value) == "" {
		return "", at(e.source, "<%s> requires a non-empty %q attribute", e.name, name)
	}
	return strings.TrimSpace(value), nil
}
