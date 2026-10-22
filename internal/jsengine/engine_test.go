package jsengine

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/dop251/goja"
	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

func TestTranslateKeepsLineNumbers(t *testing.T) {
	in := "use shop\nshow collections\nlet used = 1 // use x\n"
	out := translate(in)
	if out != "__use(\"shop\");\n__show(\"collections\");\nlet used = 1 // use x\n" {
		t.Fatalf("%q", out)
	}
}

func TestNumbersFollowMongosh(t *testing.T) {
	if v := jsNumber(5); v != int32(5) {
		t.Fatalf("5 → %T", v)
	}
	if v := jsNumber(1.5); v != 1.5 {
		t.Fatalf("1.5 → %v", v)
	}
	if v := jsNumber(1 << 40); v != float64(1<<40) {
		t.Fatalf("2^40 → %T", v)
	}
	if v := jsNumber(math.Copysign(0, -1)); v != math.Copysign(0, -1) {
		t.Fatalf("-0 must stay a double")
	}
}

func TestRoundTripKeepsTypesAndOrder(t *testing.T) {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	id := bson.NewObjectID()
	dec, _ := bson.ParseDecimal128("9.99")
	when := bson.NewDateTimeFromTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	in := bson.D{
		{Key: "z", Value: int32(1)}, {Key: "a", Value: "x"}, {Key: "_id", Value: id},
		{Key: "big", Value: int64(1) << 60}, {Key: "small", Value: int64(7)}, {Key: "price", Value: dec},
		{Key: "at", Value: when}, {Key: "tags", Value: bson.A{"p", int32(2)}}, {Key: "n", Value: nil},
		{Key: "sub", Value: bson.D{{Key: "k", Value: true}}},
	}
	out, err := fromJS(vm, toJS(vm, in))
	if err != nil {
		t.Fatal(err)
	}
	got := out.(bson.D)
	if mql.Compact(got) != mql.Compact(bson.D{
		{Key: "z", Value: int32(1)}, {Key: "a", Value: "x"}, {Key: "_id", Value: id},
		{Key: "big", Value: int64(1) << 60}, {Key: "small", Value: int32(7)}, {Key: "price", Value: dec},
		{Key: "at", Value: when}, {Key: "tags", Value: bson.A{"p", int32(2)}}, {Key: "n", Value: nil},
		{Key: "sub", Value: bson.D{{Key: "k", Value: true}}},
	}) {
		t.Fatalf("round trip changed the document:\n%s", mql.Compact(got))
	}
}

// goja passes the raw arguments only to func(goja.FunctionCall) goja.Value;
// any other return type would silently drop them.
func TestMethodsTakingArgumentsReturnAValue(t *testing.T) {
	fc := reflect.TypeOf(goja.FunctionCall{})
	val := reflect.TypeOf((*goja.Value)(nil)).Elem()
	for _, typ := range []reflect.Type{reflect.TypeOf(&Collection{}), reflect.TypeOf(&Cursor{})} {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			if m.Type.NumIn() == 2 && m.Type.In(1) == fc {
				if m.Type.NumOut() != 1 || m.Type.Out(0) != val {
					t.Errorf("%s.%s takes a FunctionCall but returns %v", typ, m.Name, m.Type)
				}
			}
		}
	}
}
