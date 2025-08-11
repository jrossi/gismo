package codesitter

import (
	"fmt"
	"strings"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	sitter "github.com/smacker/go-tree-sitter"
)

// extractGoSymbols extracts symbols from Go code
func (s *Server) extractGoSymbols(ft *FileTree) []*gismov1.Symbol {
	symbols := make([]*gismov1.Symbol, 0)
	cursor := sitter.NewTreeCursor(ft.Tree.RootNode())
	defer cursor.Close()

	s.walkGoTree(cursor, ft, &symbols, "")
	return symbols
}

// walkGoTree recursively walks the Go AST
func (s *Server) walkGoTree(cursor *sitter.TreeCursor, ft *FileTree, symbols *[]*gismov1.Symbol, parentName string) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		if name != "" {
			symbol := &gismov1.Symbol{
				Name:      name,
				Kind:      gismov1.SymbolKind_SYMBOL_KIND_FUNCTION,
				Location:  s.nodeToLocation(node, ft.Path),
				Signature: s.getFunctionSignature(node, ft.Content),
			}
			*symbols = append(*symbols, symbol)
		}

	case "method_declaration":
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		receiver := s.getNodeText(node.ChildByFieldName("receiver"), ft.Content)
		if name != "" {
			fullName := name
			if receiver != "" {
				// Extract receiver type
				receiverType := s.extractReceiverType(receiver)
				fullName = fmt.Sprintf("%s.%s", receiverType, name)
			}
			symbol := &gismov1.Symbol{
				Name:         fullName,
				Kind:         gismov1.SymbolKind_SYMBOL_KIND_METHOD,
				Location:     s.nodeToLocation(node, ft.Path),
				Signature:    s.getFunctionSignature(node, ft.Content),
				ParentSymbol: parentName,
			}
			*symbols = append(*symbols, symbol)
		}

	case "type_declaration":
		spec := node.ChildByFieldName("type_spec")
		if spec != nil {
			name := s.getNodeText(spec.ChildByFieldName("name"), ft.Content)
			if name != "" {
				typeNode := spec.ChildByFieldName("type")
				kind := s.getTypeKind(typeNode)
				symbol := &gismov1.Symbol{
					Name:     name,
					Kind:     kind,
					Location: s.nodeToLocation(node, ft.Path),
				}
				*symbols = append(*symbols, symbol)

				// Extract struct fields
				if kind == gismov1.SymbolKind_SYMBOL_KIND_STRUCT && typeNode != nil {
					s.extractStructFields(typeNode, ft, symbols, name)
				}
			}
		}

	case "const_declaration", "var_declaration":
		specs := node.ChildrenByFieldName("spec")
		for _, spec := range specs {
			names := spec.ChildrenByFieldName("name")
			for _, nameNode := range names {
				name := s.getNodeText(nameNode, ft.Content)
				if name != "" {
					kind := gismov1.SymbolKind_SYMBOL_KIND_VARIABLE
					if nodeType == "const_declaration" {
						kind = gismov1.SymbolKind_SYMBOL_KIND_CONSTANT
					}
					symbol := &gismov1.Symbol{
						Name:     name,
						Kind:     kind,
						Location: s.nodeToLocation(nameNode, ft.Path),
					}
					*symbols = append(*symbols, symbol)
				}
			}
		}
	}

	// Recursively process children
	if cursor.GoToFirstChild() {
		s.walkGoTree(cursor, ft, symbols, parentName)
		for cursor.GoToNextSibling() {
			s.walkGoTree(cursor, ft, symbols, parentName)
		}
		cursor.GoToParent()
	}
}

// extractStructFields extracts fields from a struct type
func (s *Server) extractStructFields(typeNode *sitter.Node, ft *FileTree, symbols *[]*gismov1.Symbol, structName string) {
	if typeNode.Type() != "struct_type" {
		return
	}

	fieldList := typeNode.ChildByFieldName("fields")
	if fieldList == nil {
		return
	}

	for i := 0; i < int(fieldList.ChildCount()); i++ {
		field := fieldList.Child(i)
		if field.Type() == "field_declaration" {
			names := field.ChildrenByFieldName("name")
			for _, nameNode := range names {
				name := s.getNodeText(nameNode, ft.Content)
				if name != "" {
					symbol := &gismov1.Symbol{
						Name:         fmt.Sprintf("%s.%s", structName, name),
						Kind:         gismov1.SymbolKind_SYMBOL_KIND_FIELD,
						Location:     s.nodeToLocation(nameNode, ft.Path),
						ParentSymbol: structName,
					}
					*symbols = append(*symbols, symbol)
				}
			}
		}
	}
}

// extractJSSymbols extracts symbols from JavaScript/TypeScript code
func (s *Server) extractJSSymbols(ft *FileTree) []*gismov1.Symbol {
	symbols := make([]*gismov1.Symbol, 0)
	cursor := sitter.NewTreeCursor(ft.Tree.RootNode())
	defer cursor.Close()

	s.walkJSTree(cursor, ft, &symbols, "")
	return symbols
}

// walkJSTree recursively walks the JavaScript/TypeScript AST
func (s *Server) walkJSTree(cursor *sitter.TreeCursor, ft *FileTree, symbols *[]*gismov1.Symbol, parentName string) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	switch nodeType {
	case "function_declaration", "function":
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		if name != "" {
			symbol := &gismov1.Symbol{
				Name:      name,
				Kind:      gismov1.SymbolKind_SYMBOL_KIND_FUNCTION,
				Location:  s.nodeToLocation(node, ft.Path),
				Signature: s.getFunctionSignature(node, ft.Content),
			}
			*symbols = append(*symbols, symbol)
		}

	case "arrow_function":
		// Check if it's assigned to a variable
		parent := node.Parent()
		if parent != nil && parent.Type() == "variable_declarator" {
			name := s.getNodeText(parent.ChildByFieldName("name"), ft.Content)
			if name != "" {
				symbol := &gismov1.Symbol{
					Name:      name,
					Kind:      gismov1.SymbolKind_SYMBOL_KIND_FUNCTION,
					Location:  s.nodeToLocation(node, ft.Path),
					Signature: s.getFunctionSignature(node, ft.Content),
				}
				*symbols = append(*symbols, symbol)
			}
		}

	case "class_declaration":
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		if name != "" {
			symbol := &gismov1.Symbol{
				Name:     name,
				Kind:     gismov1.SymbolKind_SYMBOL_KIND_CLASS,
				Location: s.nodeToLocation(node, ft.Path),
			}
			*symbols = append(*symbols, symbol)

			// Extract class members
			body := node.ChildByFieldName("body")
			if body != nil {
				s.extractClassMembers(body, ft, symbols, name)
			}
		}

	case "interface_declaration": // TypeScript
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		if name != "" {
			symbol := &gismov1.Symbol{
				Name:     name,
				Kind:     gismov1.SymbolKind_SYMBOL_KIND_INTERFACE,
				Location: s.nodeToLocation(node, ft.Path),
			}
			*symbols = append(*symbols, symbol)
		}

	case "variable_declaration", "lexical_declaration":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "variable_declarator" {
				name := s.getNodeText(child.ChildByFieldName("name"), ft.Content)
				if name != "" {
					kind := gismov1.SymbolKind_SYMBOL_KIND_VARIABLE
					if nodeType == "lexical_declaration" {
						// Check if const
						firstChild := node.Child(0)
						if firstChild != nil && s.getNodeText(firstChild, ft.Content) == "const" {
							kind = gismov1.SymbolKind_SYMBOL_KIND_CONSTANT
						}
					}
					symbol := &gismov1.Symbol{
						Name:     name,
						Kind:     kind,
						Location: s.nodeToLocation(child, ft.Path),
					}
					*symbols = append(*symbols, symbol)
				}
			}
		}
	}

	// Recursively process children
	if cursor.GoToFirstChild() {
		s.walkJSTree(cursor, ft, symbols, parentName)
		for cursor.GoToNextSibling() {
			s.walkJSTree(cursor, ft, symbols, parentName)
		}
		cursor.GoToParent()
	}
}

// extractClassMembers extracts members from a class body
func (s *Server) extractClassMembers(body *sitter.Node, ft *FileTree, symbols *[]*gismov1.Symbol, className string) {
	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		switch member.Type() {
		case "method_definition":
			name := s.getNodeText(member.ChildByFieldName("name"), ft.Content)
			if name != "" {
				kind := gismov1.SymbolKind_SYMBOL_KIND_METHOD
				if name == "constructor" {
					kind = gismov1.SymbolKind_SYMBOL_KIND_CONSTRUCTOR
				}
				symbol := &gismov1.Symbol{
					Name:         fmt.Sprintf("%s.%s", className, name),
					Kind:         kind,
					Location:     s.nodeToLocation(member, ft.Path),
					ParentSymbol: className,
				}
				*symbols = append(*symbols, symbol)
			}

		case "field_definition", "property_definition":
			name := s.getNodeText(member.ChildByFieldName("property"), ft.Content)
			if name != "" {
				symbol := &gismov1.Symbol{
					Name:         fmt.Sprintf("%s.%s", className, name),
					Kind:         gismov1.SymbolKind_SYMBOL_KIND_PROPERTY,
					Location:     s.nodeToLocation(member, ft.Path),
					ParentSymbol: className,
				}
				*symbols = append(*symbols, symbol)
			}
		}
	}
}

// extractPythonSymbols extracts symbols from Python code
func (s *Server) extractPythonSymbols(ft *FileTree) []*gismov1.Symbol {
	symbols := make([]*gismov1.Symbol, 0)
	cursor := sitter.NewTreeCursor(ft.Tree.RootNode())
	defer cursor.Close()

	s.walkPythonTree(cursor, ft, &symbols, "")
	return symbols
}

// walkPythonTree recursively walks the Python AST
func (s *Server) walkPythonTree(cursor *sitter.TreeCursor, ft *FileTree, symbols *[]*gismov1.Symbol, parentName string) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	switch nodeType {
	case "function_definition":
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		if name != "" {
			fullName := name
			if parentName != "" {
				fullName = fmt.Sprintf("%s.%s", parentName, name)
			}
			kind := gismov1.SymbolKind_SYMBOL_KIND_FUNCTION
			if parentName != "" {
				kind = gismov1.SymbolKind_SYMBOL_KIND_METHOD
				if name == "__init__" {
					kind = gismov1.SymbolKind_SYMBOL_KIND_CONSTRUCTOR
				}
			}
			symbol := &gismov1.Symbol{
				Name:         fullName,
				Kind:         kind,
				Location:     s.nodeToLocation(node, ft.Path),
				Signature:    s.getPythonFunctionSignature(node, ft.Content),
				ParentSymbol: parentName,
			}
			*symbols = append(*symbols, symbol)
		}

	case "class_definition":
		name := s.getNodeText(node.ChildByFieldName("name"), ft.Content)
		if name != "" {
			symbol := &gismov1.Symbol{
				Name:     name,
				Kind:     gismov1.SymbolKind_SYMBOL_KIND_CLASS,
				Location: s.nodeToLocation(node, ft.Path),
			}
			*symbols = append(*symbols, symbol)

			// Process class body
			body := node.ChildByFieldName("body")
			if body != nil {
				childCursor := sitter.NewTreeCursor(body)
				defer childCursor.Close()
				s.walkPythonTree(childCursor, ft, symbols, name)
			}
		}

	case "assignment":
		left := node.ChildByFieldName("left")
		if left != nil && left.Type() == "identifier" {
			name := s.getNodeText(left, ft.Content)
			if name != "" && !strings.HasPrefix(name, "_") { // Skip private vars
				fullName := name
				if parentName != "" {
					fullName = fmt.Sprintf("%s.%s", parentName, name)
				}
				symbol := &gismov1.Symbol{
					Name:         fullName,
					Kind:         gismov1.SymbolKind_SYMBOL_KIND_VARIABLE,
					Location:     s.nodeToLocation(left, ft.Path),
					ParentSymbol: parentName,
				}
				*symbols = append(*symbols, symbol)
			}
		}
	}

	// Recursively process children
	if cursor.GoToFirstChild() {
		s.walkPythonTree(cursor, ft, symbols, parentName)
		for cursor.GoToNextSibling() {
			s.walkPythonTree(cursor, ft, symbols, parentName)
		}
		cursor.GoToParent()
	}
}

// Helper functions

// getNodeText extracts text from a node
func (s *Server) getNodeText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	return string(content[node.StartByte():node.EndByte()])
}

// nodeToLocation converts a tree-sitter node to a Location
func (s *Server) nodeToLocation(node *sitter.Node, filePath string) *gismov1.Location {
	if node == nil {
		return nil
	}
	startPoint := node.StartPoint()
	endPoint := node.EndPoint()

	return &gismov1.Location{
		FilePath:    filePath,
		StartLine:   int32(startPoint.Row + 1), // tree-sitter uses 0-based
		StartColumn: int32(startPoint.Column + 1),
		EndLine:     int32(endPoint.Row + 1),
		EndColumn:   int32(endPoint.Column + 1),
		StartByte:   int32(node.StartByte()),
		EndByte:     int32(node.EndByte()),
	}
}

// getFunctionSignature extracts function signature from Go/JS
func (s *Server) getFunctionSignature(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Get parameters
	params := node.ChildByFieldName("parameters")
	if params != nil {
		return s.getNodeText(params, content)
	}

	return ""
}

// getPythonFunctionSignature extracts function signature from Python
func (s *Server) getPythonFunctionSignature(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Get parameters
	params := node.ChildByFieldName("parameters")
	if params != nil {
		return s.getNodeText(params, content)
	}

	return ""
}

// getTypeKind determines the SymbolKind for a type node
func (s *Server) getTypeKind(node *sitter.Node) gismov1.SymbolKind {
	if node == nil {
		return gismov1.SymbolKind_SYMBOL_KIND_CLASS
	}

	switch node.Type() {
	case "struct_type":
		return gismov1.SymbolKind_SYMBOL_KIND_STRUCT
	case "interface_type":
		return gismov1.SymbolKind_SYMBOL_KIND_INTERFACE
	case "enum_type":
		return gismov1.SymbolKind_SYMBOL_KIND_ENUM
	default:
		return gismov1.SymbolKind_SYMBOL_KIND_CLASS
	}
}

// extractReceiverType extracts the type from a method receiver
func (s *Server) extractReceiverType(receiver string) string {
	// Remove parentheses and extract type
	receiver = strings.Trim(receiver, "()")
	parts := strings.Fields(receiver)
	if len(parts) >= 2 {
		// Handle pointer receivers
		typeName := parts[len(parts)-1]
		typeName = strings.TrimPrefix(typeName, "*")
		return typeName
	}
	return ""
}
