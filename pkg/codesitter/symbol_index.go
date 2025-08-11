package codesitter

import (
	"strings"
	"sync"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// SymbolIndex provides fast symbol lookups
type SymbolIndex struct {
	mu sync.RWMutex

	// Symbols by name
	byName map[string][]*gismov1.Symbol

	// Symbols by file
	byFile map[string][]*gismov1.Symbol

	// Symbols by kind
	byKind map[gismov1.SymbolKind][]*gismov1.Symbol

	// Total count
	count int64
}

// NewSymbolIndex creates a new symbol index
func NewSymbolIndex() *SymbolIndex {
	return &SymbolIndex{
		byName: make(map[string][]*gismov1.Symbol),
		byFile: make(map[string][]*gismov1.Symbol),
		byKind: make(map[gismov1.SymbolKind][]*gismov1.Symbol),
	}
}

// Add adds a symbol to the index
func (idx *SymbolIndex) Add(symbol *gismov1.Symbol) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Index by name
	idx.byName[symbol.Name] = append(idx.byName[symbol.Name], symbol)

	// Index by file
	if symbol.Location != nil {
		idx.byFile[symbol.Location.FilePath] = append(idx.byFile[symbol.Location.FilePath], symbol)
	}

	// Index by kind
	idx.byKind[symbol.Kind] = append(idx.byKind[symbol.Kind], symbol)

	idx.count++
}

// Remove removes a symbol from the index
func (idx *SymbolIndex) Remove(symbol *gismov1.Symbol) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove from name index
	if symbols, ok := idx.byName[symbol.Name]; ok {
		idx.byName[symbol.Name] = removeSymbol(symbols, symbol)
		if len(idx.byName[symbol.Name]) == 0 {
			delete(idx.byName, symbol.Name)
		}
	}

	// Remove from file index
	if symbol.Location != nil {
		if symbols, ok := idx.byFile[symbol.Location.FilePath]; ok {
			idx.byFile[symbol.Location.FilePath] = removeSymbol(symbols, symbol)
			if len(idx.byFile[symbol.Location.FilePath]) == 0 {
				delete(idx.byFile, symbol.Location.FilePath)
			}
		}
	}

	// Remove from kind index
	if symbols, ok := idx.byKind[symbol.Kind]; ok {
		idx.byKind[symbol.Kind] = removeSymbol(symbols, symbol)
		if len(idx.byKind[symbol.Kind]) == 0 {
			delete(idx.byKind, symbol.Kind)
		}
	}

	idx.count--
}

// FindByName finds symbols by name (supports partial matching)
func (idx *SymbolIndex) FindByName(name string, exact bool) []*gismov1.Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if exact {
		return idx.byName[name]
	}

	// Partial matching
	var results []*gismov1.Symbol
	nameLower := strings.ToLower(name)
	for symbolName, symbols := range idx.byName {
		if strings.Contains(strings.ToLower(symbolName), nameLower) {
			results = append(results, symbols...)
		}
	}
	return results
}

// FindByFile finds all symbols in a file
func (idx *SymbolIndex) FindByFile(filePath string) []*gismov1.Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byFile[filePath]
}

// FindByKind finds all symbols of a specific kind
func (idx *SymbolIndex) FindByKind(kind gismov1.SymbolKind) []*gismov1.Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byKind[kind]
}

// Count returns the total number of indexed symbols
func (idx *SymbolIndex) Count() int64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.count
}

// Clear removes all symbols from the index
func (idx *SymbolIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.byName = make(map[string][]*gismov1.Symbol)
	idx.byFile = make(map[string][]*gismov1.Symbol)
	idx.byKind = make(map[gismov1.SymbolKind][]*gismov1.Symbol)
	idx.count = 0
}

// Helper function to remove a symbol from a slice
func removeSymbol(symbols []*gismov1.Symbol, target *gismov1.Symbol) []*gismov1.Symbol {
	result := make([]*gismov1.Symbol, 0, len(symbols))
	for _, s := range symbols {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}
