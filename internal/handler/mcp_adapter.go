package handler

import (
	"github.com/c-wind/mist-docs/internal/mcp"
	"github.com/c-wind/mist-docs/internal/service"
)

// mcpStore adapts the service-level store to the protocol-level interface so
// internal/mcp stays free of the service package.
type mcpStore struct{ inner service.MCPStore }

func (s mcpStore) SearchDocuments(teamID, userID, keyword string, limit int) ([]mcp.DocHit, error) {
	hits, err := s.inner.SearchDocuments(teamID, userID, keyword, limit)
	if err != nil {
		return nil, err
	}
	out := make([]mcp.DocHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, mcp.DocHit{ID: h.ID, Title: h.Title, Excerpt: h.Excerpt})
	}
	return out, nil
}

func (s mcpStore) RecentDocuments(teamID, userID string, limit int) ([]mcp.DocMeta, error) {
	docs, err := s.inner.RecentDocuments(teamID, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]mcp.DocMeta, 0, len(docs))
	for _, d := range docs {
		out = append(out, mcp.DocMeta{ID: d.ID, Title: d.Title, Type: d.Type, UpdatedAt: d.UpdatedAt})
	}
	return out, nil
}

func (s mcpStore) ReadDocument(teamID, docID string) (*mcp.DocContent, bool, error) {
	doc, found, err := s.inner.ReadDocument(teamID, docID)
	if err != nil || !found || doc == nil {
		return nil, found, err
	}
	return &mcp.DocContent{
		ID: doc.ID, Title: doc.Title, Content: doc.Content, Type: doc.Type,
		Version: doc.Version, UpdatedAt: doc.UpdatedAt,
	}, true, nil
}

func (s mcpStore) CreateDocument(teamID, title, folderID, content, userID string) (string, error) {
	return s.inner.CreateDocument(teamID, title, folderID, content, userID)
}

func (s mcpStore) CanRead(teamID, docID, userID string) (bool, error) {
	return s.inner.CanRead(teamID, docID, userID)
}

func (s mcpStore) CanCreate(teamID, userID string) (bool, string, error) {
	return s.inner.CanCreate(teamID, userID)
}

func (s mcpStore) IsMember(teamID, userID string) (bool, error) {
	return s.inner.IsMember(teamID, userID)
}

var _ mcp.Store = mcpStore{}
