package migrate

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

// Target field types.
const (
	TAuto    = "auto"
	TString  = "string"
	TInt     = "int"
	TLong    = "long"
	TDouble  = "double"
	TDecimal = "decimal"
	TBool    = "bool"
	TDate    = "date"
	TBinary  = "binary"
	TJSON    = "json"
	TUUID    = "uuid"
	TSkip    = "skip"
)

// TargetTypes lists the field types a column can map to.
var TargetTypes = []string{TAuto, TString, TInt, TLong, TDouble, TDecimal, TBool, TDate, TBinary, TJSON, TUUID, TSkip}

// SuggestTypeFor is SuggestType with driver-specific rules: SQLite stores
// every INTEGER as 64-bit, so integer columns map to long there.
func SuggestTypeFor(driver, dbType string) string {
	t := SuggestType(dbType)
	if driver == SQLite && t == TInt {
		return TLong
	}
	return t
}

// SuggestType maps a SQL column type to a BSON field type.
func SuggestType(dbType string) string {
	t := strings.ToLower(strings.TrimSpace(dbType))
	base := t
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	switch {
	case t == "tinyint(1)" || t == "bit(1)" || base == "bool" || base == "boolean":
		return TBool
	case base == "bigint" || base == "int8" || base == "bigserial":
		return TLong
	case base == "int" && strings.Contains(t, "unsigned"):
		return TLong
	case base == "integer" || base == "int" || base == "int4" || base == "int2" || base == "smallint" || base == "tinyint" ||
		base == "mediumint" || base == "serial" || base == "smallserial" || base == "year":
		return TInt
	case base == "decimal" || base == "numeric" || base == "money" || base == "dec":
		return TDecimal
	case base == "real" || base == "float" || base == "double" || base == "float4" || base == "float8" || strings.HasPrefix(t, "double precision"):
		return TDouble
	case base == "date" || strings.HasPrefix(base, "timestamp") || base == "datetime":
		return TDate
	case base == "json" || base == "jsonb":
		return TJSON
	case base == "uuid":
		return TUUID
	case base == "bytea" || strings.HasSuffix(base, "blob") || base == "binary" || base == "varbinary":
		return TBinary
	}
	// SQLite type affinity for anything else.
	switch {
	case strings.Contains(t, "int"):
		return TLong
	case strings.Contains(t, "char") || strings.Contains(t, "clob") || strings.Contains(t, "text"):
		return TString
	case strings.Contains(t, "real") || strings.Contains(t, "floa") || strings.Contains(t, "doub"):
		return TDouble
	}
	return TString
}

// ConvertValue turns a value scanned by database/sql into a BSON value of
// the requested type. TAuto keeps the driver's natural type.
func ConvertValue(v any, typ string) (any, error) {
	if v == nil {
		return nil, nil
	}
	if b, ok := v.([]byte); ok && typ != TBinary && typ != TUUID {
		v = string(b) // MySQL returns most columns as []byte
	}
	switch typ {
	case TAuto, "":
		switch t := v.(type) {
		case time.Time:
			return bson.NewDateTimeFromTime(t), nil
		case int64:
			if t >= math.MinInt32 && t <= math.MaxInt32 {
				return int32(t), nil
			}
			return t, nil
		case []byte:
			return bson.Binary{Data: append([]byte(nil), t...)}, nil
		}
		return v, nil
	case TString:
		switch t := v.(type) {
		case string:
			return t, nil
		case time.Time:
			return t.UTC().Format(time.RFC3339Nano), nil
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64), nil
		}
		return fmt.Sprint(v), nil
	case TInt, TLong:
		n, err := toInt64(v)
		if err != nil {
			return nil, err
		}
		if typ == TInt {
			if n < math.MinInt32 || n > math.MaxInt32 {
				return nil, fmt.Errorf("%d does not fit in int32 (map the column to long)", n)
			}
			return int32(n), nil
		}
		return n, nil
	case TDouble:
		switch t := v.(type) {
		case float64:
			return t, nil
		case float32:
			return float64(t), nil
		case int64:
			return float64(t), nil
		case string:
			return strconv.ParseFloat(strings.TrimSpace(t), 64)
		}
	case TDecimal:
		var s string
		switch t := v.(type) {
		case string:
			s = strings.TrimSpace(t)
		case int64:
			s = strconv.FormatInt(t, 10)
		case float64:
			s = strconv.FormatFloat(t, 'f', -1, 64)
		case *big.Rat:
			s = t.FloatString(10)
		default:
			s = fmt.Sprint(t)
		}
		s = strings.TrimPrefix(strings.ReplaceAll(s, ",", ""), "$")
		d, err := bson.ParseDecimal128(s)
		if err != nil {
			return nil, fmt.Errorf("%q is not a decimal", s)
		}
		return d, nil
	case TBool:
		switch t := v.(type) {
		case bool:
			return t, nil
		case int64:
			return t != 0, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(t)) {
			case "1", "t", "true", "y", "yes", "\x01":
				return true, nil
			case "0", "f", "false", "n", "no", "\x00", "":
				return false, nil
			}
		}
	case TDate:
		switch t := v.(type) {
		case time.Time:
			return bson.NewDateTimeFromTime(t), nil
		case string:
			tm, err := mql.ParseDate(strings.Replace(strings.TrimSpace(t), " ", "T", 1))
			if err != nil {
				if tm2, err2 := time.Parse("2006-01-02 15:04:05.999999999-07", t); err2 == nil {
					return bson.NewDateTimeFromTime(tm2), nil
				}
				return nil, fmt.Errorf("%q is not a date", t)
			}
			return bson.NewDateTimeFromTime(tm), nil
		case int64:
			return bson.DateTime(t * 1000), nil // Unix seconds (SQLite convention)
		}
	case TBinary:
		switch t := v.(type) {
		case []byte:
			return bson.Binary{Data: append([]byte(nil), t...)}, nil
		case string:
			return bson.Binary{Data: []byte(t)}, nil
		}
	case TUUID:
		switch t := v.(type) {
		case []byte:
			if len(t) == 16 {
				return bson.Binary{Subtype: 4, Data: append([]byte(nil), t...)}, nil
			}
			v = string(t)
		case [16]byte:
			return bson.Binary{Subtype: 4, Data: t[:]}, nil
		}
		if s, ok := v.(string); ok {
			b, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(s), "-", ""))
			if err == nil && len(b) == 16 {
				return bson.Binary{Subtype: 4, Data: b}, nil
			}
			return nil, fmt.Errorf("%q is not a UUID", s)
		}
	case TJSON:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("JSON column holds %T", v)
		}
		parsed, err := mql.ParseValue(s)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON: %v", err)
		}
		return parsed, nil
	}
	return nil, fmt.Errorf("cannot convert %T %v to %s", v, clip(fmt.Sprint(v)), typ)
}

func toInt64(v any) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int32:
		return int64(t), nil
	case float64:
		if t == math.Trunc(t) {
			return int64(t), nil
		}
		return 0, fmt.Errorf("%v is not a whole number", t)
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			if u, uerr := strconv.ParseUint(strings.TrimSpace(t), 10, 64); uerr == nil {
				return 0, fmt.Errorf("%d exceeds int64", u)
			}
			return 0, fmt.Errorf("%q is not an integer", t)
		}
		return n, nil
	}
	return 0, fmt.Errorf("cannot convert %T to an integer", v)
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:39] + "…"
	}
	return s
}
