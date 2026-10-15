package sqlq

import (
	"strings"
	"testing"

	"mango3s/internal/apperr"
)

func TestTranslateFind(t *testing.T) {
	cases := []struct{ sql, want string }{
		{`SELECT * FROM users`, `db.users.find({})`},
		{`select * from users;`, `db.users.find({})`},
		{`SELECT name, age FROM users WHERE age >= 21 AND status = 'A' ORDER BY age DESC, name LIMIT 10 OFFSET 20`,
			`db.users.find({"age": {"$gte": 21}, "status": "A"}, {"name": 1, "age": 1, "_id": 0}).sort({"age": -1, "name": 1}).skip(20).limit(10)`},
		{`SELECT _id, name AS n FROM users ORDER BY n`, `db.users.find({}, {"_id": 1, "n": "$name"}).sort({"name": 1})`},
		{`SELECT * FROM users WHERE age > 1 AND age < 5`, `db.users.find({"$and": [{"age": {"$gt": 1}}, {"age": {"$lt": 5}}]})`},
		{`SELECT * FROM users WHERE a = 1 OR b = 2 OR (c = 3 OR d <> 4)`,
			`db.users.find({"$or": [{"a": 1}, {"b": 2}, {"c": 3}, {"d": {"$ne": 4}}]})`},
		{`SELECT * FROM users WHERE NOT (a = 1)`, `db.users.find({"$nor": [{"a": 1}]})`},
		{`SELECT * FROM users WHERE 18 <= age`, `db.users.find({"age": {"$gte": 18}})`},
		{`SELECT * FROM users WHERE status IN ('A', 'B') AND tier NOT IN (1, 2)`,
			`db.users.find({"status": {"$in": ["A", "B"]}, "tier": {"$nin": [1, 2]}})`},
		{`SELECT * FROM users WHERE name LIKE 'Ad%'`, `db.users.find({"name": /^Ad/})`},
		{`SELECT * FROM users WHERE name ILIKE '%lov_lace%'`, `db.users.find({"name": /lov.lace/i})`},
		{`SELECT * FROM users WHERE name NOT LIKE '%x'`, `db.users.find({"name": {"$not": /x$/}})`},
		{`SELECT * FROM users WHERE email IS NULL OR phone IS NOT NULL`,
			`db.users.find({"$or": [{"email": null}, {"phone": {"$ne": null}}]})`},
		{`SELECT * FROM users WHERE score BETWEEN 1.5 AND 10`, `db.users.find({"score": {"$gte": 1.5, "$lte": 10}})`},
		{`SELECT * FROM users WHERE score NOT BETWEEN -1 AND 1`,
			`db.users.find({"$or": [{"score": {"$lt": -1}}, {"score": {"$gt": 1}}]})`},
		{`SELECT * FROM users WHERE spent > budget`, `db.users.find({"$expr": {"$gt": ["$spent", "$budget"]}})`},
		{`SELECT * FROM users WHERE address.city = 'Paris' AND "odd key" = 'x' AND ` + "`k-2`" + ` = 1`,
			`db.users.find({"address.city": "Paris", "odd key": "x", "k-2": 1})`},
		{`SELECT * FROM users WHERE _id = OBJECTID('65a1f0c2e4b0a1b2c3d4e5f6')`,
			`db.users.find({"_id": ObjectId("65a1f0c2e4b0a1b2c3d4e5f6")})`},
		{`SELECT * FROM users WHERE joined >= DATE '2024-01-01' AND active = TRUE`,
			`db.users.find({"joined": {"$gte": ISODate("2024-01-01T00:00:00.000Z")}, "active": true})`},
		{`SELECT * FROM users WHERE big = 3000000000 AND f = 2.5e3`, `db.users.find({"big": NumberLong(3000000000), "f": 2500.0})`},
		{`SELECT * FROM users WHERE name = 'O''Brien' -- comment`, `db.users.find({"name": "O'Brien"})`},
		{`SELECT * FROM users LIMIT 5, 10`, `db.users.find({}).skip(5).limit(10)`},
		{`SELECT * FROM "my-coll"`, `db.getCollection("my-coll").find({})`},
	}
	for _, c := range cases {
		tr, err := Translate(c.sql)
		if err != nil {
			t.Errorf("%s\n  error: %v", c.sql, err)
			continue
		}
		if tr.Kind != "find" {
			t.Errorf("%s: kind %s", c.sql, tr.Kind)
		}
		if got := tr.Shell(); got != c.want {
			t.Errorf("%s\n   got %s\n  want %s", c.sql, got, c.want)
		}
	}
}

func TestTranslateAggregate(t *testing.T) {
	cases := []struct{ sql, want string }{
		{`SELECT COUNT(*) FROM orders`,
			`[{"$group": {"_id": null, "count": {"$sum": 1}}}, {"$project": {"_id": 0, "count": 1}}]`},
		{`SELECT status, COUNT(*) AS n, SUM(total) AS revenue, AVG(total), MIN(total), MAX(createdAt) FROM orders WHERE total > 0 GROUP BY status HAVING COUNT(*) > 5 ORDER BY revenue DESC LIMIT 3`,
			`[{"$match": {"total": {"$gt": 0}}}, ` +
				`{"$group": {"_id": {"k0": "$status"}, "n": {"$sum": 1}, "revenue": {"$sum": "$total"}, "avg_total": {"$avg": "$total"}, "min_total": {"$min": "$total"}, "max_createdAt": {"$max": "$createdAt"}}}, ` +
				`{"$match": {"n": {"$gt": 5}}}, ` +
				`{"$project": {"_id": 0, "status": "$_id.k0", "n": 1, "revenue": 1, "avg_total": 1, "min_total": 1, "max_createdAt": 1}}, ` +
				`{"$sort": {"revenue": -1}}, {"$limit": NumberLong(3)}]`},
		{`SELECT address.city AS city, COUNT(email) FROM users GROUP BY address.city HAVING SUM(spent) >= 100 AND city <> 'X'`,
			`[{"$group": {"_id": {"k0": "$address.city"}, "count_email": {"$sum": {"$cond": [{"$gt": ["$email", null]}, 1, 0]}}, "__having0": {"$sum": "$spent"}}}, ` +
				`{"$match": {"__having0": {"$gte": 100}, "_id.k0": {"$ne": "X"}}}, ` +
				`{"$project": {"_id": 0, "city": "$_id.k0", "count_email": 1}}]`},
		{`SELECT DISTINCT country, city FROM users ORDER BY country`,
			`[{"$group": {"_id": {"k0": "$country", "k1": "$city"}}}, {"$project": {"_id": 0, "country": "$_id.k0", "city": "$_id.k1"}}, {"$sort": {"country": 1}}]`},
		{`SELECT tier, COUNT(*) FROM users GROUP BY tier ORDER BY COUNT(*) DESC OFFSET 1`,
			`[{"$group": {"_id": {"k0": "$tier"}, "count": {"$sum": 1}}}, {"$project": {"_id": 0, "tier": "$_id.k0", "count": 1}}, {"$sort": {"count": -1}}, {"$skip": NumberLong(1)}]`},
	}
	for _, c := range cases {
		tr, err := Translate(c.sql)
		if err != nil {
			t.Errorf("%s\n  error: %v", c.sql, err)
			continue
		}
		if tr.Kind != "aggregate" {
			t.Errorf("%s: kind %s", c.sql, tr.Kind)
			continue
		}
		parts := make([]string, len(tr.Pipeline))
		for i, st := range tr.Pipeline {
			parts[i] = compact(st)
		}
		if got := "[" + strings.Join(parts, ", ") + "]"; got != c.want {
			t.Errorf("%s\n   got %s\n  want %s", c.sql, got, c.want)
		}
	}
}

func TestUnsupportedAndInvalid(t *testing.T) {
	cases := map[string]string{
		``:                                      "the query is empty",
		`UPDATE users SET a = 1`:                "UPDATE is not supported",
		`DELETE FROM users`:                     "DELETE is not supported",
		`SELECT * FROM a JOIN b ON a.id = b.id`: "JOIN is not supported",
		`SELECT * FROM a, b`:                    "implicit joins",
		`SELECT * FROM users u`:                 "table aliases are not supported",
		`SELECT * FROM users WHERE id IN (SELECT id FROM x)`: "subqueries are not supported",
		`SELECT * FROM a UNION SELECT * FROM b`:              "UNION is not supported",
		`SELECT UPPER(name) FROM users`:                      "function UPPER() is not supported",
		`SELECT * FROM users WHERE LOWER(name) = 'x'`:        "function LOWER() is not supported",
		`SELECT * FROM users WHERE a + 1 > 2`:                "arithmetic",
		`SELECT name, COUNT(*) FROM users`:                   `"name" must appear in GROUP BY`,
		`SELECT * FROM users WHERE COUNT(*) > 1`:             "only allowed in SELECT, HAVING",
		`SELECT * FROM users WHERE a = NULL`:                 "IS NULL",
		`SELECT * FROM users WHERE a = b c`:                  "end of query",
		`SELECT * FROM users WHERE a = 'x`:                   "unterminated string",
		`SELECT * FROM users WHERE name = Ada`:               "",
		`SELECT * FROM users LIMIT -1`:                       "non-negative",
		`SELECT * FROM users HAVING a > 1`:                   "HAVING requires GROUP BY",
		`SELECT *, a FROM users`:                             "cannot be combined",
		`SELECT * FROM users ORDER BY COUNT(*)`:              "requires GROUP BY",
		`SELECT COUNT(DISTINCT a) FROM users`:                "COUNT(DISTINCT ...) is not supported",
		`SELECT SUM(*) FROM users`:                           "SUM(*) is not valid",
		`SELECT a FROM`:                                      "expected a field name but reached the end",
		`SELECT * FROM users WHERE _id = OBJECTID('zz')`:     "invalid ObjectId",
		`SELECT * FROM db.users`:                             "collection name only",
		`SELECT CASE WHEN a THEN 1 END FROM users`:           "CASE expressions is not supported",
	}
	for sql, want := range cases {
		_, err := Translate(sql)
		if err == nil {
			// `name = Ada` compares two fields; that is valid SQL and becomes $expr.
			if want == "" {
				continue
			}
			t.Errorf("%q: expected an error containing %q", sql, want)
			continue
		}
		if !apperr.Is(err, apperr.CodeInvalidInput) {
			t.Errorf("%q: wrong error code: %v", sql, err)
		}
		if want != "" && !strings.Contains(err.Error(), want) {
			t.Errorf("%q:\n  got %v\n want ...%s...", sql, err, want)
		}
	}
}

func TestErrorPosition(t *testing.T) {
	_, err := Translate("SELECT *\nFROM users\nWHERE a = 1 JOIN")
	if err == nil || !strings.Contains(err.Error(), "line 3, column 13") {
		t.Fatalf("position: %v", err)
	}
}

func TestLimitZero(t *testing.T) {
	tr, err := Translate("SELECT * FROM users LIMIT 0")
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.Shell(); got != `db.users.find({"$expr": false})` {
		t.Fatalf("got %s", got)
	}
}

func TestLikeToRegex(t *testing.T) {
	cases := map[string]string{
		"abc":   "^abc$",
		"abc%":  "^abc",
		"%abc":  "abc$",
		"%a.c%": `a\.c`,
		"a_c":   "^a.c$",
		`50\%`:  "^50%$",
		"%":     "^",
		"(x)%":  `^\(x\)`,
	}
	for in, want := range cases {
		if got := LikeToRegex(in); got != want {
			t.Errorf("LikeToRegex(%q) = %q, want %q", in, got, want)
		}
	}
}

func compact(v any) string { return mqlCompact(v) }
