// Package manage administers databases and collections: creating them with
// options, renaming, clearing, validation rules, indexes and statistics.
// Inputs use mongosh syntax (parsed by mql), so every BSON type is exact.
package manage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/explorer"
	"mango3s/internal/mql"
)

// CreateOptions describes a new collection.
type CreateOptions struct {
	Name             string `json:"name"`
	Capped           bool   `json:"capped"`
	SizeBytes        int64  `json:"sizeBytes"` // capped: maximum size
	MaxDocs          int64  `json:"maxDocs"`   // capped: maximum documents (optional)
	Collation        string `json:"collation"` // e.g. { locale: "en", strength: 2 }
	Validator        string `json:"validator"` // e.g. { $jsonSchema: { … } }
	ValidationLevel  string `json:"validationLevel"`
	ValidationAction string `json:"validationAction"`
	TimeField        string `json:"timeField"` // set → time series collection
	MetaField        string `json:"metaField"`
	Granularity      string `json:"granularity"`        // seconds | minutes | hours
	ExpireAfter      int64  `json:"expireAfterSeconds"` // time series / clustered TTL; 0 = none
	Clustered        bool   `json:"clustered"`          // clustered on _id (MongoDB 5.3+)
	StorageEngine    string `json:"storageEngine"`      // e.g. { wiredTiger: { configString: "block_compressor=zstd" } }
}

func parseDoc(label, src string) (bson.D, error) {
	if strings.TrimSpace(src) == "" {
		return nil, nil
	}
	d, err := mql.ParseDocument(src)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, label+": "+err.Error())
	}
	return d, nil
}

// CreateCommand builds the create command for the options.
func CreateCommand(o CreateOptions) (bson.D, error) {
	if err := explorer.ValidateName("x", o.Name); err != nil {
		return nil, err
	}
	cmd := bson.D{{Key: "create", Value: o.Name}}
	if o.Capped {
		if o.TimeField != "" || o.Clustered {
			return nil, apperr.Invalid("A capped collection can't also be a time series or clustered collection.")
		}
		if o.SizeBytes <= 0 {
			return nil, apperr.Invalid("A capped collection needs a maximum size in bytes.")
		}
		cmd = append(cmd, bson.E{Key: "capped", Value: true}, bson.E{Key: "size", Value: o.SizeBytes})
		if o.MaxDocs > 0 {
			cmd = append(cmd, bson.E{Key: "max", Value: o.MaxDocs})
		}
	}
	if o.TimeField != "" {
		ts := bson.D{{Key: "timeField", Value: o.TimeField}}
		if o.MetaField != "" {
			ts = append(ts, bson.E{Key: "metaField", Value: o.MetaField})
		}
		switch o.Granularity {
		case "":
		case "seconds", "minutes", "hours":
			ts = append(ts, bson.E{Key: "granularity", Value: o.Granularity})
		default:
			return nil, apperr.Invalid("Granularity must be seconds, minutes or hours.")
		}
		cmd = append(cmd, bson.E{Key: "timeseries", Value: ts})
	} else if o.MetaField != "" || o.Granularity != "" {
		return nil, apperr.Invalid("Set the time field to create a time series collection.")
	}
	if o.Clustered {
		cmd = append(cmd, bson.E{Key: "clusteredIndex", Value: bson.D{{Key: "key", Value: bson.D{{Key: "_id", Value: 1}}}, {Key: "unique", Value: true}}})
	}
	if o.ExpireAfter > 0 {
		if o.TimeField == "" && !o.Clustered {
			return nil, apperr.Invalid("Automatic expiry at collection level needs a time series or clustered collection; for other collections create a TTL index.")
		}
		cmd = append(cmd, bson.E{Key: "expireAfterSeconds", Value: o.ExpireAfter})
	} else if o.ExpireAfter < 0 {
		return nil, apperr.Invalid("Expiry must be a positive number of seconds.")
	}
	for _, f := range []struct{ key, label, src string }{
		{"collation", "Collation", o.Collation}, {"validator", "Validator", o.Validator}, {"storageEngine", "Storage engine", o.StorageEngine},
	} {
		d, err := parseDoc(f.label, f.src)
		if err != nil {
			return nil, err
		}
		if d != nil {
			cmd = append(cmd, bson.E{Key: f.key, Value: d})
		}
	}
	if o.ValidationLevel != "" {
		if err := oneOf("Validation level", o.ValidationLevel, "off", "strict", "moderate"); err != nil {
			return nil, err
		}
		cmd = append(cmd, bson.E{Key: "validationLevel", Value: o.ValidationLevel})
	}
	if o.ValidationAction != "" {
		if err := oneOf("Validation action", o.ValidationAction, "error", "warn", "errorAndLog"); err != nil {
			return nil, err
		}
		cmd = append(cmd, bson.E{Key: "validationAction", Value: o.ValidationAction})
	}
	return cmd, nil
}

func oneOf(label, v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return apperr.Invalid("%s must be one of %s.", label, strings.Join(allowed, ", "))
}

// CreateCollection creates a collection with options.
func CreateCollection(ctx context.Context, cl *mongo.Client, db string, o CreateOptions) error {
	if err := explorer.ValidateName(db, o.Name); err != nil {
		return err
	}
	cmd, err := CreateCommand(o)
	if err != nil {
		return err
	}
	return apperr.Check(cl.Database(db).RunCommand(ctx, cmd).Err())
}

// CreateView creates a read-only view defined by an aggregation pipeline.
func CreateView(ctx context.Context, cl *mongo.Client, db, name, viewOn, pipeline, collation string) error {
	if err := explorer.ValidateName(db, name); err != nil {
		return err
	}
	if strings.TrimSpace(viewOn) == "" {
		return apperr.Invalid("Choose the collection or view the view reads from.")
	}
	pipe := bson.A{}
	if strings.TrimSpace(pipeline) != "" {
		v, err := mql.ParseValue(pipeline)
		if err != nil {
			return apperr.Wrap(apperr.CodeInvalidInput, err, "Pipeline: "+err.Error())
		}
		a, ok := v.(bson.A)
		if !ok {
			return apperr.Invalid("The pipeline must be an array of stages, e.g. [ { $match: { active: true } } ].")
		}
		pipe = a
	}
	cmd := bson.D{{Key: "create", Value: name}, {Key: "viewOn", Value: viewOn}, {Key: "pipeline", Value: pipe}}
	c, err := parseDoc("Collation", collation)
	if err != nil {
		return err
	}
	if c != nil {
		cmd = append(cmd, bson.E{Key: "collation", Value: c})
	}
	return apperr.Check(cl.Database(db).RunCommand(ctx, cmd).Err())
}

// RenameCollection renames (or moves to another database of the same
// server) a collection. dropTarget replaces an existing target.
func RenameCollection(ctx context.Context, cl *mongo.Client, db, from, toDB, to string, dropTarget bool) error {
	if toDB == "" {
		toDB = db
	}
	if err := explorer.ValidateName(toDB, to); err != nil {
		return err
	}
	if db == toDB && from == to {
		return apperr.Invalid("The new name is the same as the current one.")
	}
	cmd := bson.D{{Key: "renameCollection", Value: db + "." + from}, {Key: "to", Value: toDB + "." + to}}
	if dropTarget {
		cmd = append(cmd, bson.E{Key: "dropTarget", Value: true})
	}
	return apperr.Check(cl.Database("admin").RunCommand(ctx, cmd).Err())
}

// Logger receives progress lines.
type Logger interface {
	Log(format string, args ...any)
}

// RenameDatabase moves every collection of db to a new database (MongoDB has
// no database rename). Collections move with their indexes and options;
// views are recreated in the new database and removed from the old one. The
// target database must not exist yet.
func RenameDatabase(ctx context.Context, cl *mongo.Client, db, to string, log Logger) error {
	if err := explorer.ValidateName(to, "x"); err != nil {
		return err
	}
	if db == to {
		return apperr.Invalid("The new name is the same as the current one.")
	}
	if db == "admin" || db == "local" || db == "config" {
		return apperr.Invalid("System databases can't be renamed.")
	}
	existing, err := cl.Database(to).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return apperr.From(err)
	}
	if len(existing) > 0 {
		return apperr.New(apperr.CodeConflict, "Database %q already exists; choose a new name.", to)
	}
	cur, err := cl.Database(db).ListCollections(ctx, bson.D{})
	if err != nil {
		return apperr.From(err)
	}
	var infos []struct {
		Name    string `bson:"name"`
		Type    string `bson:"type"`
		Options bson.D `bson:"options"`
	}
	if err := cur.All(ctx, &infos); err != nil {
		return apperr.From(err)
	}
	// Check everything can move before moving anything.
	for _, ci := range infos {
		if ci.Type == "timeseries" {
			return apperr.New(apperr.CodeUnsupported, "%s.%s is a time series collection, which the server can't rename. Use Copy database instead, then drop the old database.", db, ci.Name)
		}
	}
	var views []int
	for i, ci := range infos {
		if ci.Type == "view" {
			views = append(views, i)
			continue
		}
		if strings.HasPrefix(ci.Name, "system.") {
			continue // system.views is recreated with the views; buckets move with their time series
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := RenameCollection(ctx, cl, db, ci.Name, to, ci.Name, false); err != nil {
			return fmt.Errorf("moving %s: %w (collections moved so far are in %s)", ci.Name, err, to)
		}
		log.Log("moved %s.%s → %s.%s", db, ci.Name, to, ci.Name)
	}
	// Views may be defined on other views: create in dependency order.
	pending := views
	for len(pending) > 0 {
		var next []int
		progress := false
		for _, i := range pending {
			cmd := append(bson.D{{Key: "create", Value: infos[i].Name}}, infos[i].Options...)
			if err := cl.Database(to).RunCommand(ctx, cmd).Err(); err != nil {
				next = append(next, i)
				continue
			}
			progress = true
			if err := cl.Database(db).Collection(infos[i].Name).Drop(ctx); err != nil {
				return apperr.From(err)
			}
			log.Log("recreated view %s.%s", to, infos[i].Name)
		}
		if !progress {
			names := []string{}
			for _, i := range next {
				names = append(names, infos[i].Name)
			}
			return fmt.Errorf("could not recreate views %s in %s", strings.Join(names, ", "), to)
		}
		pending = next
	}
	left, err := cl.Database(db).ListCollectionNames(ctx, bson.D{{Key: "name", Value: bson.D{{Key: "$not", Value: bson.Regex{Pattern: "^system\\."}}}}})
	if err == nil && len(left) == 0 {
		return apperr.Check(cl.Database(db).Drop(ctx))
	}
	return nil
}

// ClearCollection deletes every document, keeping the collection, its
// indexes, options and validator.
func ClearCollection(ctx context.Context, c *mongo.Collection) (int64, error) {
	r, err := c.DeleteMany(ctx, bson.D{})
	if err != nil {
		return 0, apperr.From(err)
	}
	return r.DeletedCount, nil
}

// Validation is a collection's schema validation.
type Validation struct {
	Validator string `json:"validator"` // mongosh syntax; "" = none
	Level     string `json:"level"`     // off | strict | moderate
	Action    string `json:"action"`    // error | warn | errorAndLog
}

// CollectionOptions returns the listCollections entry of a collection.
func CollectionOptions(ctx context.Context, db *mongo.Database, coll string) (typ string, opts bson.D, info bson.D, err error) {
	cur, err := db.ListCollections(ctx, bson.D{{Key: "name", Value: coll}})
	if err != nil {
		return "", nil, nil, apperr.From(err)
	}
	var rows []struct {
		Type    string `bson:"type"`
		Options bson.D `bson:"options"`
		Info    bson.D `bson:"info"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return "", nil, nil, apperr.From(err)
	}
	if len(rows) == 0 {
		return "", nil, nil, apperr.New(apperr.CodeNotFound, "Collection %s.%s does not exist.", db.Name(), coll)
	}
	return rows[0].Type, rows[0].Options, rows[0].Info, nil
}

// GetValidation reads a collection's validation rules.
func GetValidation(ctx context.Context, db *mongo.Database, coll string) (Validation, error) {
	_, opts, _, err := CollectionOptions(ctx, db, coll)
	if err != nil {
		return Validation{}, err
	}
	v := Validation{Level: "strict", Action: "error"}
	for _, e := range opts {
		switch e.Key {
		case "validator":
			if d, ok := e.Value.(bson.D); ok && len(d) > 0 {
				v.Validator = mql.Pretty(d)
			}
		case "validationLevel":
			v.Level, _ = e.Value.(string)
		case "validationAction":
			v.Action, _ = e.Value.(string)
		}
	}
	return v, nil
}

// SetValidation replaces a collection's validation rules (collMod). An
// empty validator removes validation.
func SetValidation(ctx context.Context, db *mongo.Database, coll string, v Validation) error {
	validator, err := parseDoc("Validator", v.Validator)
	if err != nil {
		return err
	}
	if validator == nil {
		validator = bson.D{}
	}
	if err := oneOf("Validation level", v.Level, "off", "strict", "moderate"); err != nil {
		return err
	}
	if err := oneOf("Validation action", v.Action, "error", "warn", "errorAndLog"); err != nil {
		return err
	}
	cmd := bson.D{{Key: "collMod", Value: coll}, {Key: "validator", Value: validator}, {Key: "validationLevel", Value: v.Level}, {Key: "validationAction", Value: v.Action}}
	return apperr.Check(db.RunCommand(ctx, cmd).Err())
}

// ValidationReport counts documents that fail a validator, so a rule can be
// checked before it's applied.
type ValidationReport struct {
	Checked int64    `json:"checked"`
	Failing int64    `json:"failing"`
	Sample  []string `json:"sample"` // _id values of failing documents
}

// CheckValidator counts existing documents that don't match a validator
// (up to limit documents examined; 0 = all).
func CheckValidator(ctx context.Context, c *mongo.Collection, validator string) (ValidationReport, error) {
	v, err := parseDoc("Validator", validator)
	if err != nil {
		return ValidationReport{}, err
	}
	rep := ValidationReport{Sample: []string{}}
	if v == nil {
		return rep, nil
	}
	total, err := c.CountDocuments(ctx, bson.D{})
	if err != nil {
		return rep, apperr.From(err)
	}
	rep.Checked = total
	notMatching := bson.D{{Key: "$nor", Value: bson.A{v}}}
	n, err := c.CountDocuments(ctx, notMatching)
	if err != nil {
		return rep, apperr.Wrap(apperr.CodeInvalidInput, err, "The validator can't be evaluated: "+apperr.From(err).Message)
	}
	rep.Failing = n
	cur, err := c.Find(ctx, notMatching, findIDs(5))
	if err == nil {
		defer cur.Close(context.Background())
		for cur.Next(ctx) {
			if id, err := cur.Current.LookupErr("_id"); err == nil {
				var x any
				if id.Unmarshal(&x) == nil {
					rep.Sample = append(rep.Sample, mql.Compact(x))
				}
			}
		}
	}
	return rep, nil
}

// ---- indexes ------------------------------------------------------------------

// Index describes one index, including usage statistics when available.
type Index struct {
	Name        string `json:"name"`
	Keys        string `json:"keys"`
	Spec        string `json:"spec"` // the whole index specification
	Unique      bool   `json:"unique"`
	Sparse      bool   `json:"sparse"`
	Hidden      bool   `json:"hidden"`
	TTL         *int64 `json:"ttl,omitempty"` // expireAfterSeconds
	Partial     string `json:"partial,omitempty"`
	Collation   string `json:"collation,omitempty"`
	Size        int64  `json:"size"`
	Accesses    int64  `json:"accesses"`
	AccessSince string `json:"accessSince,omitempty"`
}

// ListIndexes returns every index with its options, size and usage.
func ListIndexes(ctx context.Context, db *mongo.Database, coll string) ([]Index, error) {
	c := db.Collection(coll)
	cur, err := c.Indexes().List(ctx)
	if err != nil {
		return nil, apperr.From(err)
	}
	var specs []bson.D
	if err := cur.All(ctx, &specs); err != nil {
		return nil, apperr.From(err)
	}
	sizes := map[string]int64{}
	var st struct {
		IndexSizes bson.M `bson:"indexSizes"`
	}
	if db.RunCommand(ctx, bson.D{{Key: "collStats", Value: coll}}).Decode(&st) == nil {
		for k, v := range st.IndexSizes {
			sizes[k] = toInt64(v)
		}
	}
	usage := map[string][2]any{}
	if ucur, err := c.Aggregate(ctx, mongo.Pipeline{{{Key: "$indexStats", Value: bson.D{}}}}); err == nil {
		var rows []struct {
			Name     string `bson:"name"`
			Accesses struct {
				Ops   any           `bson:"ops"`
				Since bson.DateTime `bson:"since"`
			} `bson:"accesses"`
		}
		if ucur.All(ctx, &rows) == nil {
			for _, r := range rows {
				prev := usage[r.Name]
				n := toInt64(r.Accesses.Ops)
				if p, ok := prev[0].(int64); ok {
					n += p
				}
				usage[r.Name] = [2]any{n, r.Accesses.Since.Time().UTC().Format("2006-01-02 15:04")}
			}
		}
	}
	out := make([]Index, 0, len(specs))
	for _, s := range specs {
		ix := Index{Spec: mql.Pretty(s)}
		for _, e := range s {
			switch e.Key {
			case "name":
				ix.Name, _ = e.Value.(string)
			case "key":
				ix.Keys = mql.Compact(e.Value)
			case "unique":
				ix.Unique, _ = e.Value.(bool)
			case "sparse":
				ix.Sparse, _ = e.Value.(bool)
			case "hidden":
				ix.Hidden, _ = e.Value.(bool)
			case "expireAfterSeconds":
				n := toInt64(e.Value)
				ix.TTL = &n
			case "partialFilterExpression":
				ix.Partial = mql.Compact(e.Value)
			case "collation":
				ix.Collation = mql.Compact(e.Value)
			}
		}
		ix.Size = sizes[ix.Name]
		if u, ok := usage[ix.Name]; ok {
			ix.Accesses, _ = u[0].(int64)
			ix.AccessSince, _ = u[1].(string)
		}
		out = append(out, ix)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name == "_id_" && out[j].Name != "_id_" })
	return out, nil
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	}
	return 0
}

// IndexOptions describes a new index.
type IndexOptions struct {
	Keys               string `json:"keys"` // e.g. { email: 1 } or { title: "text" }
	Name               string `json:"name"`
	Unique             bool   `json:"unique"`
	Sparse             bool   `json:"sparse"`
	Hidden             bool   `json:"hidden"`
	ExpireAfter        *int64 `json:"expireAfterSeconds"` // TTL
	Partial            string `json:"partialFilterExpression"`
	Collation          string `json:"collation"`
	WildcardProjection string `json:"wildcardProjection"`
	Extra              string `json:"extra"` // any other options, e.g. { weights: { title: 10 }, default_language: "english" }
}

// IndexSpec builds a createIndexes entry.
func IndexSpec(o IndexOptions) (bson.D, error) {
	keys, err := parseDoc("Keys", o.Keys)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, apperr.Invalid("Enter the index keys, e.g. { email: 1 }.")
	}
	spec := bson.D{{Key: "key", Value: keys}}
	name := strings.TrimSpace(o.Name)
	if name == "" {
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s_%v", k.Key, k.Value))
		}
		name = strings.Join(parts, "_")
	}
	spec = append(spec, bson.E{Key: "name", Value: name})
	if o.Unique {
		spec = append(spec, bson.E{Key: "unique", Value: true})
	}
	if o.Sparse {
		spec = append(spec, bson.E{Key: "sparse", Value: true})
	}
	if o.Hidden {
		spec = append(spec, bson.E{Key: "hidden", Value: true})
	}
	if o.ExpireAfter != nil {
		if *o.ExpireAfter < 0 {
			return nil, apperr.Invalid("TTL must be zero or more seconds.")
		}
		if len(keys) != 1 {
			return nil, apperr.Invalid("A TTL index must be on a single date field.")
		}
		spec = append(spec, bson.E{Key: "expireAfterSeconds", Value: *o.ExpireAfter})
	}
	for _, f := range []struct{ key, label, src string }{
		{"partialFilterExpression", "Partial filter", o.Partial}, {"collation", "Collation", o.Collation}, {"wildcardProjection", "Wildcard projection", o.WildcardProjection},
	} {
		d, err := parseDoc(f.label, f.src)
		if err != nil {
			return nil, err
		}
		if d != nil {
			spec = append(spec, bson.E{Key: f.key, Value: d})
		}
	}
	extra, err := parseDoc("Other options", o.Extra)
	if err != nil {
		return nil, err
	}
	for _, e := range extra {
		if lookupKey(spec, e.Key) {
			return nil, apperr.Invalid("Other options repeat %q; set it once.", e.Key)
		}
		spec = append(spec, e)
	}
	return spec, nil
}

func lookupKey(d bson.D, k string) bool {
	for _, e := range d {
		if e.Key == k {
			return true
		}
	}
	return false
}

// CreateIndex builds an index and returns its name.
func CreateIndex(ctx context.Context, db *mongo.Database, coll string, o IndexOptions) (string, error) {
	spec, err := IndexSpec(o)
	if err != nil {
		return "", err
	}
	err = db.RunCommand(ctx, bson.D{{Key: "createIndexes", Value: coll}, {Key: "indexes", Value: bson.A{spec}}}).Err()
	if err != nil {
		return "", apperr.From(err)
	}
	name, _ := spec[1].Value.(string)
	return name, nil
}

// DropIndex drops an index by name. The _id index can't be dropped.
func DropIndex(ctx context.Context, db *mongo.Database, coll, name string) error {
	if name == "_id_" {
		return apperr.Invalid("The _id index can't be dropped.")
	}
	return apperr.Check(db.RunCommand(ctx, bson.D{{Key: "dropIndexes", Value: coll}, {Key: "index", Value: name}}).Err())
}

// ModifyIndex hides/unhides an index or changes its TTL (collMod, MongoDB
// 4.4+ for hidden indexes).
func ModifyIndex(ctx context.Context, db *mongo.Database, coll, name string, hidden *bool, ttl *int64) error {
	if name == "_id_" {
		return apperr.Invalid("The _id index can't be modified.")
	}
	idx := bson.D{{Key: "name", Value: name}}
	if hidden != nil {
		idx = append(idx, bson.E{Key: "hidden", Value: *hidden})
	}
	if ttl != nil {
		if *ttl < 0 {
			return apperr.Invalid("TTL must be zero or more seconds.")
		}
		idx = append(idx, bson.E{Key: "expireAfterSeconds", Value: *ttl})
	}
	if len(idx) == 1 {
		return errors.New("nothing to change")
	}
	return apperr.Check(db.RunCommand(ctx, bson.D{{Key: "collMod", Value: coll}, {Key: "index", Value: idx}}).Err())
}

// ---- information ---------------------------------------------------------------

// CollectionDetails is the collection information dialog.
type CollectionDetails struct {
	Type       string            `json:"type"`
	Options    string            `json:"options"` // mongosh syntax
	UUID       string            `json:"uuid,omitempty"`
	ReadOnly   bool              `json:"readOnly"`
	Stats      map[string]string `json:"stats"` // label → formatted value
	Count      int64             `json:"count"`
	Size       int64             `json:"size"`
	Storage    int64             `json:"storageSize"`
	IndexSize  int64             `json:"totalIndexSize"`
	AvgObjSize int64             `json:"avgObjSize"`
	Capped     bool              `json:"capped"`
	Sharded    bool              `json:"sharded"`
	Shell      string            `json:"shell"` // the command to recreate it
	// Sharding is the shard key and chunk distribution (sharded clusters only).
	Sharding *CollectionSharding `json:"sharding,omitempty"`
}

// Details reads a collection's options, metadata and statistics.
func Details(ctx context.Context, db *mongo.Database, coll string) (CollectionDetails, error) {
	typ, opts, info, err := CollectionOptions(ctx, db, coll)
	if err != nil {
		return CollectionDetails{}, err
	}
	d := CollectionDetails{Type: typ, Options: mql.Pretty(opts), Stats: map[string]string{}}
	if typ == "" {
		d.Type = "collection"
	}
	for _, e := range info {
		switch e.Key {
		case "uuid":
			if b, ok := e.Value.(bson.Binary); ok {
				d.UUID = fmt.Sprintf("%x-%x-%x-%x-%x", b.Data[0:4], b.Data[4:6], b.Data[6:8], b.Data[8:10], b.Data[10:16])
			}
		case "readOnly":
			d.ReadOnly, _ = e.Value.(bool)
		}
	}
	create := append(bson.D{}, opts...)
	d.Shell = fmt.Sprintf("db.createCollection(%s, %s)", mql.Quote(coll), mql.Compact(create))
	if d.Type == "view" {
		return d, nil
	}
	var st bson.M
	if serr := db.RunCommand(ctx, bson.D{{Key: "collStats", Value: coll}}).Decode(&st); serr == nil {
		d.Count = toInt64(st["count"])
		d.Size = toInt64(st["size"])
		d.Storage = toInt64(st["storageSize"])
		d.IndexSize = toInt64(st["totalIndexSize"])
		d.AvgObjSize = toInt64(st["avgObjSize"])
		d.Capped, _ = st["capped"].(bool)
		d.Sharded, _ = st["sharded"].(bool)
		for _, k := range []string{"nindexes", "max", "maxSize", "freeStorageSize", "scaleFactor"} {
			if v, ok := st[k]; ok {
				d.Stats[k] = fmt.Sprint(v)
			}
		}
	} else {
		n, err := db.Collection(coll).EstimatedDocumentCount(ctx)
		if err != nil {
			return d, apperr.From(err)
		}
		d.Count = n
		d.Stats["note"] = "Storage statistics are unavailable: " + apperr.From(serr).Message
	}
	return d, nil
}

// DatabaseDetails summarizes a database (dbStats).
type DatabaseDetails struct {
	Collections int64 `json:"collections"`
	Views       int64 `json:"views"`
	Objects     int64 `json:"objects"`
	DataSize    int64 `json:"dataSize"`
	StorageSize int64 `json:"storageSize"`
	Indexes     int64 `json:"indexes"`
	IndexSize   int64 `json:"indexSize"`
	AvgObjSize  int64 `json:"avgObjSize"`
	FSUsed      int64 `json:"fsUsedSize"`
	FSTotal     int64 `json:"fsTotalSize"`
	// Sharding is the primary shard, shards and balancer (sharded clusters only).
	Sharding *DatabaseSharding `json:"sharding,omitempty"`
}

// DBDetails runs dbStats.
func DBDetails(ctx context.Context, db *mongo.Database) (DatabaseDetails, error) {
	var st bson.M
	if err := db.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&st); err != nil {
		return DatabaseDetails{}, apperr.From(err)
	}
	return DatabaseDetails{
		Collections: toInt64(st["collections"]), Views: toInt64(st["views"]), Objects: toInt64(st["objects"]),
		DataSize: toInt64(st["dataSize"]), StorageSize: toInt64(st["storageSize"]), Indexes: toInt64(st["indexes"]),
		IndexSize: toInt64(st["indexSize"]), AvgObjSize: toInt64(st["avgObjSize"]), FSUsed: toInt64(st["fsUsedSize"]), FSTotal: toInt64(st["fsTotalSize"]),
	}, nil
}

func findIDs(n int64) *options.FindOptionsBuilder {
	return options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}).SetLimit(n)
}
