package provenance

import (
	"reflect"
	"testing"
)

func TestParseUnifiedDiff_SimpleAdd(t *testing.T) {
	diff := `diff --git a/a.txt b/a.txt
index 111..222 100644
--- a/a.txt
+++ b/a.txt
@@ -1,1 +1,2 @@
 hello
+world
`
	got := ParseUnifiedDiff(diff)
	want := []LineRange{{File: "a.txt", Start: 2, End: 2, Origin: OriginAgent}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestParseUnifiedDiff_NewFile(t *testing.T) {
	diff := `diff --git a/b.txt b/b.txt
new file mode 100644
index 000..333
--- /dev/null
+++ b/b.txt
@@ -0,0 +1,3 @@
+x
+y
+z
`
	got := ParseUnifiedDiff(diff)
	want := []LineRange{{File: "b.txt", Start: 1, End: 3, Origin: OriginAgent}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestParseUnifiedDiff_AdjacentHunksMerge(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,0 +1,1 @@
+one
@@ -2,0 +2,1 @@
+two
`
	got := ParseUnifiedDiff(diff)
	want := []LineRange{{File: "a.go", Start: 1, End: 2, Origin: OriginAgent}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestParseUnifiedDiff_GapNotMerged(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,0 +1,1 @@
+one
@@ -5,0 +5,1 @@
+five
`
	got := ParseUnifiedDiff(diff)
	want := []LineRange{
		{File: "a.go", Start: 1, End: 1, Origin: OriginAgent},
		{File: "a.go", Start: 5, End: 5, Origin: OriginAgent},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestParseUnifiedDiff_MultiFile(t *testing.T) {
	diff := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,0 +1,1 @@
+hello
diff --git a/b.txt b/b.txt
--- a/b.txt
+++ b/b.txt
@@ -1,0 +1,2 @@
+foo
+bar
`
	got := ParseUnifiedDiff(diff)
	want := []LineRange{
		{File: "a.txt", Start: 1, End: 1, Origin: OriginAgent},
		{File: "b.txt", Start: 1, End: 2, Origin: OriginAgent},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestParseUnifiedDiff_NoNewlineMarkerIgnored(t *testing.T) {
	diff := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,0 +1,1 @@
+hello
\ No newline at end of file
`
	got := ParseUnifiedDiff(diff)
	want := []LineRange{{File: "a.txt", Start: 1, End: 1, Origin: OriginAgent}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
