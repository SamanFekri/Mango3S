// Package masking replaces sensitive values in documents. Methods that must
// stay consistent across documents and runs (hash, email, randomize, date
// shift, noise) are keyed with an HMAC secret, so equal inputs mask to equal
// outputs (joins still work) while the originals cannot be recovered without
// the key. Every method declares which BSON types it supports; anything else
// is handled by the rule's onUnsupported policy, never silently passed
// through.
package masking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/docpath"
	"mango3s/internal/mql"
)

// Methods.
const (
	MethodRedact    = "redact"    // any → "[REDACTED]"
	MethodPartial   = "partial"   // string → "*******1234"
	MethodHash      = "hash"      // any → keyed SHA-256 hex (deterministic)
	MethodEmail     = "email"     // string → "u_<hash>@<domain>"
	MethodRandomize = "randomize" // string/int → same shape, deterministic
	MethodDateShift = "dateShift" // date → shifted by a deterministic number of days
	MethodNoise     = "noise"     // number → ±percent, type kept, deterministic
	MethodNull      = "null"      // any → null
	MethodRemove    = "remove"    // delete the field
	MethodConstant  = "constant"  // any → fixed value
)

// MethodInfo documents a method for the UI.
type MethodInfo struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Types         []string `json:"types"` // supported BSON types; "any" for all
	Deterministic bool     `json:"deterministic"`
	Description   string   `json:"description"`
}

// Methods lists the methods and their supported types.
var Methods = []MethodInfo{
	{MethodRedact, "Redact", []string{"any"}, true, `Replace with the string "[REDACTED]".`},
	{MethodPartial, "Partial", []string{"string"}, true, "Mask all but the last N characters."},
	{MethodHash, "Hash", []string{"any"}, true, "Keyed SHA-256; equal values give equal hashes, so joins keep working."},
	{MethodEmail, "Email", []string{"string"}, true, "Pseudonymize the local part, keep the domain."},
	{MethodRandomize, "Randomize", []string{"string", "int32", "int64"}, true, "Same length and character classes (letters, digits), different characters."},
	{MethodDateShift, "Date shift", []string{"date"}, true, "Shift by up to ±N days, consistently per value."},
	{MethodNoise, "Numeric noise", []string{"int32", "int64", "double", "decimal"}, true, "Multiply by 1 ± up to P percent, keeping the type."},
	{MethodNull, "Null", []string{"any"}, true, "Replace with null."},
	{MethodRemove, "Remove", []string{"any"}, true, "Delete the field."},
	{MethodConstant, "Constant", []string{"any"}, true, "Replace with a fixed value."},
}

// Rule masks one field path (arrays of documents are traversed; arrays at
// the path are masked element by element).
type Rule struct {
	Field         string  `json:"field"`
	Method        string  `json:"method"`
	KeepLast      int     `json:"keepLast"`
	MaskChar      string  `json:"maskChar"`
	Value         string  `json:"value"` // constant: shell-syntax literal
	MaxDays       int     `json:"maxDays"`
	Percent       float64 `json:"percent"`
	OnUnsupported string  `json:"onUnsupported"` // error (default) | null | skip
}

// Masker applies compiled rules.
type Masker struct {
	rules  []Rule
	values []any
	key    []byte
}

// Compile validates rules. key must be at least 16 bytes.
func Compile(rules []Rule, key []byte) (*Masker, error) {
	if len(rules) == 0 {
		return nil, apperr.Invalid("Add at least one masking rule.")
	}
	if len(key) < 16 {
		return nil, apperr.Invalid("The masking key is missing or too short.")
	}
	rules = append([]Rule(nil), rules...) // defaults below must not leak into the caller's slice
	m := &Masker{rules: rules, values: make([]any, len(rules)), key: key}
	seen := map[string]bool{}
	for i, r := range rules {
		label := fmt.Sprintf("Rule %d (%s)", i+1, r.Field)
		if strings.TrimSpace(r.Field) == "" {
			return nil, apperr.Invalid("Rule %d: choose a field.", i+1)
		}
		if r.Field == "_id" && r.Method != MethodHash {
			return nil, apperr.Invalid("%s: _id can only be hashed (it must stay unique and present).", label)
		}
		if seen[r.Field] {
			return nil, apperr.Invalid("%s: the field has more than one rule.", label)
		}
		seen[r.Field] = true
		if info(r.Method) == nil {
			return nil, apperr.Invalid("%s: unknown method %q.", label, r.Method)
		}
		switch r.OnUnsupported {
		case "", "error", "null", "skip":
		default:
			return nil, apperr.Invalid("%s: onUnsupported must be error, null or skip.", label)
		}
		switch r.Method {
		case MethodConstant:
			v, err := mql.ParseValue(r.Value)
			if err != nil {
				return nil, apperr.Invalid("%s: value: %v", label, err)
			}
			m.values[i] = v
		case MethodPartial:
			if r.KeepLast < 0 || r.KeepLast > 8 {
				return nil, apperr.Invalid("%s: keep between 0 and 8 trailing characters.", label)
			}
		case MethodDateShift:
			if r.MaxDays <= 0 {
				rules[i].MaxDays = 30
			}
		case MethodNoise:
			if r.Percent <= 0 || r.Percent > 100 {
				rules[i].Percent = 10
			}
		}
	}
	return m, nil
}

func info(method string) *MethodInfo {
	for i := range Methods {
		if Methods[i].ID == method {
			return &Methods[i]
		}
	}
	return nil
}

// UnsupportedError reports a value whose type the rule's method can't mask.
type UnsupportedError struct {
	Field, Method, Type string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s: method %s does not support %s values", e.Field, e.Method, e.Type)
}

// Apply masks a copy of d.
func (m *Masker) Apply(d bson.D) (bson.D, bool, error) {
	out := docpath.CloneDoc(d)
	for i, r := range m.rules {
		var err error
		out, _, err = docpath.Update(out, r.Field, func(v any) (any, bool, error) {
			if r.Method == MethodRemove {
				return nil, false, nil
			}
			if arr, ok := v.(bson.A); ok && !wholeValue(r.Method) {
				res := make(bson.A, len(arr))
				for j, el := range arr {
					nv, keep, err := m.maskOne(r, i, el)
					if err != nil {
						return nil, false, err
					}
					if !keep {
						nv = nil
					}
					res[j] = nv
				}
				return res, true, nil
			}
			return m.maskOne(r, i, v)
		})
		if err != nil {
			return nil, false, err
		}
	}
	return out, !mql.Equal(out, d), nil
}

// wholeValue methods replace an array as one value instead of per element.
func wholeValue(method string) bool {
	return method == MethodNull || method == MethodConstant || method == MethodRedact || method == MethodHash
}

func (m *Masker) maskOne(r Rule, i int, v any) (any, bool, error) {
	if v == nil {
		return nil, true, nil // nothing sensitive in null
	}
	if _, ok := v.(bson.Null); ok {
		return v, true, nil
	}
	nv, err := m.mask(r, i, v)
	if err == nil {
		return nv, true, nil
	}
	if _, unsupported := err.(*UnsupportedError); unsupported {
		switch r.OnUnsupported {
		case "null":
			return nil, true, nil
		case "skip":
			return v, true, nil
		}
	}
	return nil, false, err
}

func (m *Masker) mac(parts ...[]byte) []byte {
	h := hmac.New(sha256.New, m.key)
	for _, p := range parts {
		h.Write(p)
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}

func canonical(v any) []byte {
	data, err := bson.Marshal(bson.D{{Key: "v", Value: v}})
	if err != nil {
		return []byte(mql.Compact(v))
	}
	return data
}

func (m *Masker) rng(field string, v any) *rand.Rand {
	sum := m.mac([]byte(field), canonical(v))
	return rand.New(rand.NewPCG(binary.BigEndian.Uint64(sum[:8]), binary.BigEndian.Uint64(sum[8:16])))
}

func (m *Masker) mask(r Rule, i int, v any) (any, error) {
	unsupported := func() error { return &UnsupportedError{Field: r.Field, Method: r.Method, Type: mql.TypeName(v)} }
	switch r.Method {
	case MethodRedact:
		return "[REDACTED]", nil
	case MethodNull:
		return nil, nil
	case MethodConstant:
		return docpath.Clone(m.values[i]), nil
	case MethodHash:
		// Keyed by the method only (not the field) so the same value hashes
		// identically in different collections and fields: joins keep working.
		return hex.EncodeToString(m.mac([]byte("hash"), canonical(v)))[:32], nil
	case MethodPartial:
		s, ok := v.(string)
		if !ok {
			return nil, unsupported()
		}
		ch := r.MaskChar
		if ch == "" {
			ch = "*"
		}
		runes := []rune(s)
		keep := min(r.KeepLast, len(runes)/2) // never reveal more than half
		return strings.Repeat(ch, len(runes)-keep) + string(runes[len(runes)-keep:]), nil
	case MethodEmail:
		s, ok := v.(string)
		if !ok {
			return nil, unsupported()
		}
		at := strings.LastIndexByte(s, '@')
		if at <= 0 || at == len(s)-1 {
			return nil, fmt.Errorf("%s: %q is not an email address", r.Field, clip(s))
		}
		local := hex.EncodeToString(m.mac([]byte("email"), []byte(strings.ToLower(s[:at]))))[:10]
		return "u_" + local + s[at:], nil
	case MethodRandomize:
		rg := m.rng(r.Field, v)
		switch t := v.(type) {
		case string:
			return randomizeString(t, rg), nil
		case int32:
			return int32(randomizeInt(int64(t), rg, math.MaxInt32)), nil
		case int64:
			return randomizeInt(t, rg, math.MaxInt64), nil
		}
		return nil, unsupported()
	case MethodDateShift:
		dt, ok := v.(bson.DateTime)
		if !ok {
			return nil, unsupported()
		}
		days := m.rng(r.Field, v).IntN(2*r.MaxDays+1) - r.MaxDays
		if days == 0 {
			days = 1
		}
		return bson.NewDateTimeFromTime(dt.Time().Add(time.Duration(days) * 24 * time.Hour)), nil
	case MethodNoise:
		f := 1 + (m.rng(r.Field, v).Float64()*2-1)*r.Percent/100
		switch t := v.(type) {
		case int32:
			return int32(math.Round(float64(t) * f)), nil
		case int64:
			return int64(math.Round(float64(t) * f)), nil
		case float64:
			return t * f, nil
		case bson.Decimal128:
			x, err := strconv.ParseFloat(t.String(), 64)
			if err != nil {
				return nil, unsupported()
			}
			d, err := bson.ParseDecimal128(strconv.FormatFloat(x*f, 'f', 2, 64))
			if err != nil {
				return nil, unsupported()
			}
			return d, nil
		}
		return nil, unsupported()
	}
	return nil, fmt.Errorf("unknown method %s", r.Method)
}

func randomizeString(s string, rg *rand.Rand) string {
	const lower, upper, digits = "abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789"
	var b strings.Builder
	for _, c := range s {
		switch {
		case unicode.IsDigit(c):
			b.WriteByte(digits[rg.IntN(10)])
		case unicode.IsUpper(c):
			b.WriteByte(upper[rg.IntN(26)])
		case unicode.IsLetter(c):
			b.WriteByte(lower[rg.IntN(26)])
		default:
			b.WriteRune(c) // keep separators: phone and card formats survive
		}
	}
	return b.String()
}

// randomizeInt keeps the sign and number of digits.
func randomizeInt(n int64, rg *rand.Rand, limit int64) int64 {
	neg := n < 0
	digits := len(strconv.FormatInt(n, 10))
	if neg {
		digits--
	}
	lo := int64(math.Pow10(digits - 1))
	if digits == 1 {
		lo = 0
	}
	hi := int64(math.Pow10(digits)) - 1
	if hi > limit || hi <= 0 {
		hi = limit
	}
	v := lo + rg.Int64N(hi-lo+1)
	if v == n || v == -n {
		v = lo + (v-lo+1)%(hi-lo+1)
	}
	if neg {
		return -v
	}
	return v
}

func clip(s string) string {
	if len(s) > 20 {
		return s[:19] + "…"
	}
	return s
}
