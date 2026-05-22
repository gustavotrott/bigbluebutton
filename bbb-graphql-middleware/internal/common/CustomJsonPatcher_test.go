package common

import (
	"encoding/json"
	"testing"

	evanphxjsonpatch "github.com/evanphx/json-patch"
)

func TestCreateJsonPatchUsesSingleMoveForRotation(t *testing.T) {
	original := []map[string]interface{}{
		testUser("a"),
		testUser("b"),
		testUser("c"),
		testUser("d"),
	}
	modified := []map[string]interface{}{
		testUser("b"),
		testUser("c"),
		testUser("d"),
		testUser("a"),
	}

	modifiedJSON := mustMarshalJSON(t, modified)
	patch := CreateJsonPatchFromMaps(original, modified, modifiedJSON, "userId")
	assertPatchApplies(t, original, modifiedJSON, patch)

	ops := mustDecodePatchOps(t, patch)
	if len(ops) != 1 || ops[0]["op"] != "move" {
		t.Fatalf("expected one move operation, got %s", patch)
	}
}

func TestCreateJsonPatchMovesBeforeAddRemoveWhenShorter(t *testing.T) {
	original := []map[string]interface{}{
		testUser("a"),
		testUser("b"),
		testUser("c"),
	}
	modified := []map[string]interface{}{
		testUser("b"),
		testUser("d"),
		testUser("a"),
	}

	modifiedJSON := mustMarshalJSON(t, modified)
	patch := CreateJsonPatchFromMaps(original, modified, modifiedJSON, "userId")
	assertPatchApplies(t, original, modifiedJSON, patch)

	ops := mustDecodePatchOps(t, patch)
	if len(ops) != 3 {
		t.Fatalf("expected move/add/remove in three operations, got %d operations: %s", len(ops), patch)
	}
	if containsPatchOp(ops, "replace") {
		t.Fatalf("expected ID-aware structural operations instead of replacing rows: %s", patch)
	}
}

func TestCreateJsonPatchAppliesValueUpdatesAfterMovingUsers(t *testing.T) {
	original := []map[string]interface{}{
		testUser("a", map[string]interface{}{"name": "Ann", "voice": map[string]interface{}{"muted": false}}),
		testUser("b", map[string]interface{}{"name": "Bob", "voice": map[string]interface{}{"muted": false}}),
		testUser("c", map[string]interface{}{"name": "Cam", "voice": map[string]interface{}{"muted": false}}),
	}
	modified := []map[string]interface{}{
		testUser("b", map[string]interface{}{"name": "Bob", "voice": map[string]interface{}{"muted": true}}),
		testUser("d", map[string]interface{}{"name": "Dee", "voice": map[string]interface{}{"muted": false}}),
		testUser("a", map[string]interface{}{"name": "Ana", "voice": map[string]interface{}{"muted": false}}),
	}

	modifiedJSON := mustMarshalJSON(t, modified)
	patch := CreateJsonPatchFromMaps(original, modified, modifiedJSON, "userId")
	assertPatchApplies(t, original, modifiedJSON, patch)

	ops := mustDecodePatchOps(t, patch)
	for _, expectedOp := range []string{"move", "add", "remove", "replace"} {
		if !containsPatchOp(ops, expectedOp) {
			t.Fatalf("expected %q operation in patch: %s", expectedOp, patch)
		}
	}
}

func TestCreateJsonPatchKeepsStableUsersWhenPresenterMovesFromFirstToLast(t *testing.T) {
	original := []map[string]interface{}{
		presenterTestUser("a", true),
		presenterTestUser("b", false),
		presenterTestUser("c", false),
		presenterTestUser("d", false),
		presenterTestUser("e", false),
	}
	modified := []map[string]interface{}{
		presenterTestUser("e", true),
		presenterTestUser("b", false),
		presenterTestUser("c", false),
		presenterTestUser("d", false),
		presenterTestUser("a", false),
	}

	modifiedJSON := mustMarshalJSON(t, modified)
	patch := CreateJsonPatchFromMaps(original, modified, modifiedJSON, "userId")
	assertPatchApplies(t, original, modifiedJSON, patch)

	ops := mustDecodePatchOps(t, patch)
	moveCount := countPatchOps(ops, "move")
	replaceCount := countPatchOps(ops, "replace")
	if moveCount != 2 || replaceCount != 2 {
		t.Fatalf("expected two presenter replaces and two moves, got %d replaces and %d moves: %s", replaceCount, moveCount, patch)
	}
}

func TestLongestCommonSubsequenceKeepsStableMiddleUsers(t *testing.T) {
	lcs := longestCommonSubsequence(
		[]string{"a", "b", "c", "d", "e"},
		[]string{"e", "b", "c", "d", "a"},
	)
	expected := []string{"b", "c", "d"}
	if !stringSlicesEqual(lcs, expected) {
		t.Fatalf("expected LCS %v, got %v", expected, lcs)
	}
}

func BenchmarkLongestCommonSubsequenceLargeList(b *testing.B) {
	originalIDs, modifiedIDs := shiftedIDLists(2000)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lcs := longestCommonSubsequence(originalIDs, modifiedIDs)
		if len(lcs) != 1998 {
			b.Fatalf("expected LCS length 1998, got %d", len(lcs))
		}
	}
}

func TestValidateIfShouldUseCustomJsonPatchParsesUserListInsteadOfUsingRawSubstring(t *testing.T) {
	original := []byte(`[{"__typename":"user","userId":"a"},{"__typename":"user","userId":"b"}]`)
	modified := []byte(`[{"__typename":"user","userId":"b"},{"__typename":"user","userId":"a"}]`)

	shouldUseCustomPatch, patch := ValidateIfShouldUseCustomJsonPatch(original, modified, "userId")
	if !shouldUseCustomPatch {
		t.Fatal("expected custom patch to be enabled for user list with reordered JSON fields")
	}

	assertPatchApplies(t, GetMapFromByte(original), modified, patch)
}

func TestValidateIfShouldUseCustomJsonPatchRejectsItemsWithoutIDs(t *testing.T) {
	original := []byte(`[{"userId":"a","__typename":"user"},{"__typename":"user"}]`)
	modified := []byte(`[{"userId":"a","__typename":"user"},{"__typename":"user"}]`)

	shouldUseCustomPatch, patch := ValidateIfShouldUseCustomJsonPatch(original, modified, "userId")
	if shouldUseCustomPatch || patch != nil {
		t.Fatalf("expected custom patch to be disabled for user list with missing IDs, got %t and %s", shouldUseCustomPatch, patch)
	}
}

func testUser(id string, extraFields ...map[string]interface{}) map[string]interface{} {
	user := map[string]interface{}{
		"userId":     id,
		"name":       id,
		"__typename": "user",
	}
	for _, fields := range extraFields {
		for key, value := range fields {
			user[key] = value
		}
	}

	return user
}

func presenterTestUser(id string, presenter bool) map[string]interface{} {
	return testUser(id, map[string]interface{}{
		"presenter": presenter,
		"avatar":    "https://example.test/avatar/" + id,
		"color":     "color-" + id,
		"extId":     "external-" + id,
		"role":      "viewer-" + id,
	})
}

func shiftedIDLists(length int) ([]string, []string) {
	originalIDs := make([]string, length)
	for i := range originalIDs {
		originalIDs[i] = string(rune('a'+i%26)) + "-" + string(rune('a'+(i/26)%26)) + "-" + string(rune('a'+(i/676)%26))
	}

	modifiedIDs := make([]string, 0, length)
	modifiedIDs = append(modifiedIDs, originalIDs[length-1])
	modifiedIDs = append(modifiedIDs, originalIDs[1:length-1]...)
	modifiedIDs = append(modifiedIDs, originalIDs[0])

	return originalIDs, modifiedIDs
}

func mustMarshalJSON(t *testing.T, value interface{}) []byte {
	t.Helper()

	valueJSON, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	return valueJSON
}

func mustDecodePatchOps(t *testing.T, patch []byte) []map[string]interface{} {
	t.Helper()

	var ops []map[string]interface{}
	if err := json.Unmarshal(patch, &ops); err != nil {
		t.Fatalf("failed to decode patch %s: %v", patch, err)
	}

	return ops
}

func assertPatchApplies(t *testing.T, original []map[string]interface{}, modifiedJSON []byte, patch []byte) {
	t.Helper()

	patchedJSON, err := ApplyPatch(original, patch)
	if err != nil {
		t.Fatalf("failed to apply patch %s: %v", patch, err)
	}
	if !evanphxjsonpatch.Equal(patchedJSON, modifiedJSON) {
		t.Fatalf("patch %s produced %s, want %s", patch, patchedJSON, modifiedJSON)
	}
}

func containsPatchOp(ops []map[string]interface{}, op string) bool {
	for _, patchOp := range ops {
		if patchOp["op"] == op {
			return true
		}
	}

	return false
}

func countPatchOps(ops []map[string]interface{}, op string) int {
	count := 0
	for _, patchOp := range ops {
		if patchOp["op"] == op {
			count++
		}
	}

	return count
}

func stringSlicesEqual(a []string, b []string) bool {
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
