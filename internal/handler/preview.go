package handler

import (
	"regexp"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
)

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// previewExcerpt turns stored HTML into a short plain line for document cards.
// Sheet JSON and empty bodies stay blank so the card can show a grid instead.
func previewExcerpt(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return ""
	}
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	const max = 96
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

func firstNonEmpty(parts ...string) string {
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			return s
		}
	}
	return ""
}

type docTagJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func tagsByDocument(ids []string) map[string][]docTagJSON {
	out := map[string][]docTagJSON{}
	if len(ids) == 0 || database.DB == nil {
		return out
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := database.DB.Query(
		`SELECT dt.document_id, t.id, t.name, IFNULL(t.color,'')
		 FROM md_doc_tags dt JOIN md_tags t ON dt.tag_id = t.id
		 WHERE dt.document_id IN (`+placeholders+`)
		 ORDER BY t.name`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var docID string
		var tag docTagJSON
		if err := rows.Scan(&docID, &tag.ID, &tag.Name, &tag.Color); err != nil {
			continue
		}
		out[docID] = append(out[docID], tag)
	}
	return out
}
