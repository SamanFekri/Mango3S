// Package model holds DTOs shared by the services and the UI facade.
package model

import (
	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

// Document is a BSON document prepared for the UI in two encodings.
type Document struct {
	// ID is the _id rendered in shell syntax (e.g. ObjectId("...")); it is
	// passed back verbatim to identify the document for edits and deletes.
	// Empty when the document has no _id (e.g. aggregation output).
	ID string `json:"id"`
	// Text is the pretty, round-trippable shell-syntax rendering.
	Text string `json:"text"`
	// JSON is canonical Extended JSON (exact types), used by the tree and
	// table views.
	JSON string `json:"json"`
}

// NewDocument converts a decoded BSON document.
func NewDocument(d bson.D) Document {
	doc := Document{Text: mql.Pretty(d), JSON: mql.CanonicalJSON(d)}
	if id, ok := mql.Lookup(d, "_id"); ok {
		doc.ID = mql.Compact(id)
	}
	return doc
}

// Page is one page of results from a find, aggregation or shell command.
type Page struct {
	Documents  []Document `json:"documents"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
	HasMore    bool       `json:"hasMore"`
	DurationMs int64      `json:"durationMs"`
	Notice     string     `json:"notice,omitempty"`
}

// DefaultPageSize applies when a request does not specify one.
const DefaultPageSize = 50

// Paging is the effective skip/limit for one page of a query that may itself
// carry a user skip and limit.
type Paging struct {
	Skip  int64
	Limit int64 // documents to return for this page; 0 means the page is empty
}

// ComputePaging maps (user skip, user limit, page, pageSize) to the window the
// server should return. userLimit <= 0 means unlimited.
func ComputePaging(userSkip, userLimit int64, page, pageSize int) Paging {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if userSkip < 0 {
		userSkip = 0
	}
	offset := int64(page) * int64(pageSize)
	limit := int64(pageSize)
	if userLimit > 0 {
		remaining := userLimit - offset
		if remaining <= 0 {
			return Paging{Skip: userSkip + offset, Limit: 0}
		}
		if remaining < limit {
			limit = remaining
		}
	}
	return Paging{Skip: userSkip + offset, Limit: limit}
}

// HasMore reports whether another page exists given the number of documents
// fetched with a probe of Limit+1.
func (p Paging) HasMore(fetched int, userSkip, userLimit int64) bool {
	if int64(fetched) <= p.Limit {
		return false
	}
	if userLimit > 0 {
		return p.Skip-userSkip+p.Limit < userLimit
	}
	return true
}
