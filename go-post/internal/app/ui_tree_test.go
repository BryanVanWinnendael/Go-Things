package app

import (
	"go-post/internal/core"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCollectionContextMenus(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.RightClick("Starter kit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.ctxMenu.open {
		t.Fatal("right-clicking a collection did not open the themed menu")
	}
	menu := menuLabels(a.ctxMenu.items)
	for _, item := range []string{"New request", "New folder", "Rename", "Delete"} {
		if !contains(menu, item) {
			t.Fatalf("collection menu %v lacks %q", menu, item)
		}
	}
	if a.collapsed["col_starter"] {
		t.Fatal("right-clicking a collection collapsed it")
	}
	tt.ClickAt(4, 4)
	tt.Frame()

	if err := tt.RightClick("HTTPBin / Post"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	menu = menuLabels(a.ctxMenu.items)
	for _, item := range []string{"Open request", "Rename request", "Move to", "Delete request"} {
		if !contains(menu, item) {
			t.Fatalf("request menu %v lacks %q", menu, item)
		}
	}
	tt.ClickAt(4, 4)
	tt.Frame()
	if a.ctxMenu.open {
		t.Fatal("clicking outside did not close the context menu")
	}
}

func TestThemedContextMenuAction(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.RightClick("HTTPBin / Post"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	before := len(a.ws().Collection("col_starter").Requests)
	if err := tt.Click("Duplicate"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ctxMenu.open {
		t.Fatal("choosing a menu item left the menu open")
	}
	if got := len(a.ws().Collection("col_starter").Requests); got != before+1 {
		t.Fatalf("duplicate left %d requests, want %d", got, before+1)
	}

	if err := tt.Click("Request actions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.ctxMenu.open {
		t.Fatal("the request actions button did not open the themed menu")
	}
	tt.ClickAt(4, 4)
	tt.Frame()

	if err := tt.RightClick("HTTPBin / Post"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.ctxMenu.open {
		t.Fatal("Escape did not close the context menu")
	}

	a.page = "collections"
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Collection actions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.ctxMenu.open {
		t.Fatal("the collection actions button did not open the themed menu")
	}
	tt.ClickAt(4, 4)
	tt.Frame()
}

func TestThemedSubmenuMovesRequest(t *testing.T) {
	a := testApp(t)
	a.ws().Collections = append(a.ws().Collections, core.Collection{ID: "col_second", Name: "Second"})
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.RightClick("HTTPBin / Post"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	move, ok := tt.Find("Move to")
	if !ok {
		t.Fatal("Move to item not found")
	}
	tt.Move(move.X+move.W/2, move.Y+move.H/2)
	tt.Frame()
	tt.Frame()
	sub, ok := tt.Find("Move to menu")
	if !ok {
		t.Fatal("themed submenu not found")
	}
	tt.ClickAt(sub.X+30, sub.Y+sub.H-15)
	tt.Frame()
	tt.Frame()

	target := a.ws().Collection("col_second")
	if target == nil || len(target.Requests) != 1 || target.Requests[0].ID != "req_httpbin_post" {
		t.Fatalf("submenu did not move the request: %+v", target)
	}
	if a.ctxMenu.open {
		t.Fatal("choosing a submenu item left the menu open")
	}
}

func TestThemedMethodMenu(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Method"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.ctxMenu.open {
		t.Fatal("the method button did not open the themed menu")
	}
	if err := tt.Click("POST"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, _ := a.activeRequest()
	if r == nil || r.Method != "POST" {
		t.Fatalf("method is %+v, want POST", r)
	}
	if a.ctxMenu.open {
		t.Fatal("choosing a method left the menu open")
	}
}

func TestCollapseAndExpandAllButton(t *testing.T) {
	a := testApp(t)
	// Give the tree at least one nested folder to collapse as well.
	a.newCollectionIn("Auth", "col_starter")
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Collapse all"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, col := range a.ws().Collections {
		if !a.collapsed[col.ID] {
			t.Fatalf("%q stayed expanded after Collapse all", col.Name)
		}
	}

	if err := tt.Click("Expand all"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, col := range a.ws().Collections {
		if a.collapsed[col.ID] {
			t.Fatalf("%q stayed collapsed after Expand all", col.Name)
		}
	}
}

func TestNewFolderAndNestedCounts(t *testing.T) {
	a := testApp(t)
	a.collapsed["col_starter"] = true
	a.newCollectionIn("Auth", "col_starter")
	children := a.ws().CollectionChildren("col_starter")
	if len(children) != 1 || children[0].Name != "Auth" {
		t.Fatalf("new folder was not created: %+v", children)
	}
	if a.collapsed["col_starter"] {
		t.Fatal("new folder left its parent collapsed")
	}
	if got := a.ws().CollectionRequestCount("col_starter"); got != 3 {
		t.Fatalf("parent request count is %d, want 3", got)
	}

	child := children[0]
	childID := child.ID
	r := core.BlankRequest("Nested request")
	child.Requests = append(child.Requests, r)
	a.ws().OpenTab(r.ID)
	a.deleteCollection("col_starter")
	if a.ws().Collection("col_starter") != nil || a.ws().Collection(childID) != nil {
		t.Fatal("deleting a collection left its folders behind")
	}
	for _, id := range a.ws().OpenTabs {
		if id == r.ID {
			t.Fatal("deleting a collection left a nested request tab open")
		}
	}
}

func TestDragRequestBetweenCollections(t *testing.T) {
	a := testApp(t)
	a.ws().Collections = append(a.ws().Collections, core.Collection{ID: "col_second", Name: "Second"})
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	from, ok := tt.Find("HTTPBin / Post")
	if !ok {
		t.Fatal("request row not found")
	}
	to, ok := tt.Find("Second")
	if !ok {
		t.Fatal("target collection not found")
	}
	dragBetween(tt, from, to)
	tt.Frame()
	tt.Frame()

	if a.ws().Collection("col_starter") != nil && len(a.ws().Collection("col_starter").Requests) != 2 {
		t.Fatalf("source collection still has %d requests, want 2", len(a.ws().Collection("col_starter").Requests))
	}
	target := a.ws().Collection("col_second")
	if target == nil || len(target.Requests) != 1 || target.Requests[0].ID != "req_httpbin_post" {
		t.Fatalf("request was not moved into the target collection: %+v", target)
	}
}

func TestDragRequestReordersWithinCollection(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	from, ok := tt.Find("HTTPBin / Get")
	if !ok {
		t.Fatal("source request not found")
	}
	to, ok := tt.Find("GitHub / Zen")
	if !ok {
		t.Fatal("target request not found")
	}
	// The lower half of the last row means "place it after me".
	dragRequestTo(tt, from, to, 0.8)

	got := requestIDs(a.ws().Collection("col_starter"))
	want := []string{"req_httpbin_post", "req_github_zen", "req_httpbin_get"}
	if !equalStrings(got, want) {
		t.Fatalf("order is %v, want %v", got, want)
	}
}

func TestDragRequestIntoFolderAtPosition(t *testing.T) {
	a := testApp(t)
	a.ws().Collections = append(a.ws().Collections, core.Collection{
		ID: "col_second", Name: "Second",
		Requests: []core.Request{
			{ID: "req_first", Name: "First", Method: "GET"},
			{ID: "req_second", Name: "Second request", Method: "GET"},
		},
	})
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	from, ok := tt.Find("HTTPBin / Get")
	if !ok {
		t.Fatal("source request not found")
	}
	to, ok := tt.Find("First")
	if !ok {
		t.Fatal("target row not found")
	}
	// The upper half of the first row means "place it before me".
	dragRequestTo(tt, from, to, 0.2)

	got := requestIDs(a.ws().Collection("col_second"))
	want := []string{"req_httpbin_get", "req_first", "req_second"}
	if !equalStrings(got, want) {
		t.Fatalf("order is %v, want %v", got, want)
	}
	if ids := requestIDs(a.ws().Collection("col_starter")); len(ids) != 2 {
		t.Fatalf("source collection has %d requests, want 2", len(ids))
	}
}

func dragRequestTo(tt *ui.Tester, from, to ui.Rect, yFraction float32) {
	tx := to.X + to.W/2
	ty := to.Y + to.H*yFraction
	fx := from.X + from.W/2
	fy := from.Y + from.H/2
	tt.Press(fx, fy)
	for k := 1; k <= 5; k++ {
		f := float32(k) / 5
		tt.Move(fx+(tx-fx)*f, fy+(ty-fy)*f)
		tt.Frame()
	}
	tt.Release(tx, ty)
	tt.Frame()
	tt.Frame()
}

func requestIDs(col *core.Collection) []string {
	if col == nil {
		return nil
	}
	out := make([]string, 0, len(col.Requests))
	for _, r := range col.Requests {
		out = append(out, r.ID)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDragCollectionIntoAnother(t *testing.T) {
	a := testApp(t)
	a.ws().Collections = append(a.ws().Collections, core.Collection{ID: "col_second", Name: "Second"})
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	from, ok := tt.Find("Starter kit")
	if !ok {
		t.Fatal("source collection not found")
	}
	to, ok := tt.Find("Second")
	if !ok {
		t.Fatal("target collection not found")
	}
	dragBetween(tt, from, to)
	tt.Frame()
	tt.Frame()

	if got := a.ws().Collection("col_starter").ParentID; got != "col_second" {
		t.Fatalf("folder parent is %q, want col_second", got)
	}
	if a.canMoveCollection("col_second", "col_starter") {
		t.Fatal("a folder was allowed to move into its own child")
	}
	a.moveCollection("col_second", "col_starter")
	if got := a.ws().Collection("col_second").ParentID; got != "" {
		t.Fatalf("cycle move changed the parent to %q", got)
	}
}

func dragBetween(tt *ui.Tester, from, to ui.Rect) {
	tt.Press(from.X+from.W/2, from.Y+from.H/2)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(from.X+from.W/2+(to.X+to.W/2-from.X-from.W/2)*f, from.Y+from.H/2+(to.Y+to.H/2-from.Y-from.H/2)*f)
	}
	tt.Release(to.X+to.W/2, to.Y+to.H/2)
}

func menuLabels(items []contextMenuItem) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		if item.Separator {
			labels = append(labels, "-")
		} else {
			labels = append(labels, item.Label)
		}
	}
	return labels
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
