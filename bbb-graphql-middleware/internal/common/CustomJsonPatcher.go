package common

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	evanphxjsonpatch "github.com/evanphx/json-patch"
	"github.com/mattbaird/jsonpatch"
	log "github.com/sirupsen/logrus"
)

type jsonPatchCandidate struct {
	patch []byte
}

type patchOperation map[string]interface{}

type patchOperationCandidate struct {
	name string
	ops  []patchOperation
}

const maxItemsForGreedyMoveCandidates = 64

func ValidateIfShouldUseCustomJsonPatch(original []byte, modified []byte, idFieldName string) (bool, []byte) {
	// Temporarily use CustomPatch only for UserList (testing feature)
	modifiedMap := GetMapFromByte(modified)
	if modifiedMap == nil {
		return false, nil
	}

	if len(modifiedMap) <= 1 {
		return false, nil
	}

	if !isUserList(modifiedMap) {
		return false, nil
	}

	if hasInvalidOrDuplicatedId(modifiedMap, idFieldName) {
		return false, nil
	}

	// Test Original Data
	originalMap := GetMapFromByte(original)
	if originalMap == nil {
		return false, nil
	}

	if len(originalMap) <= 1 {
		return false, nil
	}

	if hasInvalidOrDuplicatedId(originalMap, idFieldName) {
		return false, nil
	}

	return true, CreateJsonPatchFromMaps(originalMap, modifiedMap, modified, idFieldName)
}

func isUserList(items []map[string]interface{}) bool {
	if len(items) == 0 {
		return false
	}

	typeName, existsTypeName := items[len(items)-1]["__typename"].(string)
	return existsTypeName && typeName == "user"
}

func hasInvalidOrDuplicatedId(items []map[string]interface{}, idFieldName string) bool {
	seen := make(map[string]bool)
	for _, item := range items {
		idValue, existsIdField := item[idFieldName].(string)
		if !existsIdField {
			return true
		}
		if _, exists := seen[idValue]; exists {
			return true
		}
		seen[idValue] = true
	}

	return false
}

func CreateJsonPatch(original []byte, modified []byte, idFieldName string) []byte {
	originalMap := GetMapFromByte(original)
	modifiedMap := GetMapFromByte(modified)

	return CreateJsonPatchFromMaps(originalMap, modifiedMap, modified, idFieldName)
}

func CreateJsonPatchFromMaps(original []map[string]interface{}, modified []map[string]interface{}, modifiedJson []byte, idFieldName string) []byte {
	var candidates []jsonPatchCandidate

	// Generate several valid strategies and select by serialized size, not by operation count.
	if legacyPatch := CreateLegacyJsonPatchFromMaps(original, modified, modifiedJson, idFieldName); len(legacyPatch) > 0 {
		candidates = append(candidates, jsonPatchCandidate{patch: legacyPatch})
	}

	alternativePatch := PatchUsingMattbairdJsonpatch(original, modified)
	alternativePatchJson, _ := json.Marshal(alternativePatch)
	if len(alternativePatchJson) > 0 {
		candidates = append(candidates, jsonPatchCandidate{patch: alternativePatchJson})
	}

	candidates = append(candidates, CreateIDAwarePatchCandidates(original, modified, idFieldName)...)

	if bestPatch, bestPatchExists := selectShortestValidPatch(original, modifiedJson, candidates); bestPatchExists {
		return bestPatch
	}

	log.Error("It was not able to recreate the target data using any JSON patch candidate")
	return alternativePatchJson
}

func CreateLegacyJsonPatchFromMaps(original []map[string]interface{}, modified []map[string]interface{}, modifiedJson []byte, idFieldName string) []byte {
	// CREATE PATCHES FOR OPERATION "REPLACE"
	replacesPatches, originalWithReplaces := CreateReplacePatches(original, modified, idFieldName)

	// CREATE PATCHES FOR OPERATION "ADD" and "REMOVE"
	addRemovePatches := CreateAddRemovePatches(originalWithReplaces, modified, idFieldName)

	mergedPatch := append(replacesPatches, addRemovePatches...)
	mergedPatchJson, _ := json.Marshal(mergedPatch)

	originalWithPatches, applyErr := ApplyPatch(original, mergedPatchJson)
	if applyErr == nil && evanphxjsonpatch.Equal(originalWithPatches, modifiedJson) {
		return mergedPatchJson
	}
	if applyErr != nil {
		return nil
	}

	// CREATE PATCHES FOR OPERATION "MOVE"
	movesPatches, moveErr := CreateMovePatches(originalWithPatches, modifiedJson, idFieldName)
	if moveErr != nil {
		return nil
	}

	mergedPatchJson, applyErr = MergePatches(mergedPatchJson, movesPatches)
	if applyErr != nil {
		return nil
	}

	originalWithPatches, applyErr = ApplyPatch(original, mergedPatchJson)
	if applyErr == nil && evanphxjsonpatch.Equal(originalWithPatches, modifiedJson) {
		return mergedPatchJson
	}

	return nil
}

func CreateReplacePatches(original []map[string]interface{}, modified []map[string]interface{}, idFieldName string) ([]jsonpatch.JsonPatchOperation, []map[string]interface{}) {
	var replacesListAsMap []map[string]interface{}

	for _, originalItem := range original {
		if id, existsIdField := originalItem[idFieldName].(string); existsIdField {
			itemInNewList := findItemWithId(modified, id, originalItem, idFieldName)

			replacesListAsMap = append(replacesListAsMap, itemInNewList)
		}
	}

	return PatchUsingMattbairdJsonpatch(original, replacesListAsMap), replacesListAsMap
}

func CreateAddRemovePatches(original []map[string]interface{}, modified []map[string]interface{}, idFieldName string) []jsonpatch.JsonPatchOperation {
	hasSameIDs := false
	addedFakeItem := false
	if len(original) == len(modified) {
		hasSameIDs = true
		for i := range original {
			if original[i][idFieldName] != modified[i][idFieldName] {
				hasSameIDs = false
				break
			}
		}

		if !hasSameIDs {
			modified = append(modified, modified[len(modified)-1])
			addedFakeItem = true
		}
	}
	patch := PatchUsingMattbairdJsonpatch(original, modified)

	if addedFakeItem {
		patch = patch[0 : len(patch)-1]
	}

	return patch
}

func CreateIDAwarePatchCandidates(original []map[string]interface{}, modified []map[string]interface{}, idFieldName string) []jsonPatchCandidate {
	originalIDs, originalIDsOk := idsFromItems(original, idFieldName)
	modifiedIDs, modifiedIDsOk := idsFromItems(modified, idFieldName)
	if !originalIDsOk || !modifiedIDsOk {
		return nil
	}

	structuralCandidates := createStructuralPatchCandidates(modified, originalIDs, modifiedIDs)
	valuePatchesAtOriginalIndexes := createValuePatchOperationsAtOriginalIndexes(original, modified, idFieldName)
	valuePatchesAtTargetIndexes := createValuePatchOperationsAtTargetIndexes(original, modified, idFieldName)

	// Try value updates both before and after structural changes because index paths can differ.
	candidates := make([]jsonPatchCandidate, 0, len(structuralCandidates)*2)
	for _, structuralCandidate := range structuralCandidates {
		appendPatchOperationCandidate(
			&candidates,
			combinePatchOperations(structuralCandidate.ops, valuePatchesAtTargetIndexes),
		)
		appendPatchOperationCandidate(
			&candidates,
			combinePatchOperations(valuePatchesAtOriginalIndexes, structuralCandidate.ops),
		)
	}

	return candidates
}

func createStructuralPatchCandidates(modified []map[string]interface{}, originalIDs []string, modifiedIDs []string) []patchOperationCandidate {
	candidates := make([]patchOperationCandidate, 0)
	if ops, ok := createRemoveThenTargetForwardPatch(originalIDs, modifiedIDs, modified); ok {
		candidates = append(candidates, patchOperationCandidate{name: "remove-then-target-forward", ops: ops})
	}
	if ops, ok := createTargetForwardThenRemovePatch(originalIDs, modifiedIDs, modified); ok {
		candidates = append(candidates, patchOperationCandidate{name: "target-forward-then-remove", ops: ops})
	}

	candidates = append(candidates, createRemoveThenReorderThenAddPatchCandidates(originalIDs, modifiedIDs, modified)...)
	candidates = append(candidates, createRemoveThenAddThenReorderPatchCandidates(originalIDs, modifiedIDs, modified)...)

	return dedupePatchOperationCandidates(candidates)
}

func createRemoveThenTargetForwardPatch(originalIDs []string, modifiedIDs []string, modified []map[string]interface{}) ([]patchOperation, bool) {
	targetIDs := stringSet(modifiedIDs)
	currentIDs, ops := removeIDsNotInTarget(originalIDs, targetIDs)
	currentIDs, ops, ok := applyTargetForwardAddsAndMoves(currentIDs, modifiedIDs, modified, ops)
	if !ok || !idsEqual(currentIDs, modifiedIDs) {
		return nil, false
	}

	return ops, true
}

func createTargetForwardThenRemovePatch(originalIDs []string, modifiedIDs []string, modified []map[string]interface{}) ([]patchOperation, bool) {
	currentIDs, ops, ok := applyTargetForwardAddsAndMoves(cloneStrings(originalIDs), modifiedIDs, modified, nil)
	if !ok {
		return nil, false
	}

	currentIDs, ops = removeIDsNotInTargetWithExistingOps(currentIDs, stringSet(modifiedIDs), ops)
	if !idsEqual(currentIDs, modifiedIDs) {
		return nil, false
	}

	return ops, true
}

func createRemoveThenReorderThenAddPatchCandidates(originalIDs []string, modifiedIDs []string, modified []map[string]interface{}) []patchOperationCandidate {
	originalIDSet := stringSet(originalIDs)
	currentIDs, removeOps := removeIDsNotInTarget(originalIDs, stringSet(modifiedIDs))
	targetCommonIDs := make([]string, 0, len(modifiedIDs))
	for _, id := range modifiedIDs {
		if originalIDSet[id] {
			targetCommonIDs = append(targetCommonIDs, id)
		}
	}

	moveCandidates := createMovePatchOperationCandidatesForIDs(currentIDs, targetCommonIDs)
	candidates := make([]patchOperationCandidate, 0, len(moveCandidates))
	for _, moveCandidate := range moveCandidates {
		ops := combinePatchOperations(removeOps, moveCandidate.ops)
		currentIDsAfterMoves := cloneStrings(targetCommonIDs)

		for targetIndex, id := range modifiedIDs {
			if originalIDSet[id] {
				continue
			}
			ops = append(ops, addPatchOperation(fmt.Sprintf("/%d", targetIndex), modified[targetIndex]))
			currentIDsAfterMoves = insertString(currentIDsAfterMoves, targetIndex, id)
		}

		if idsEqual(currentIDsAfterMoves, modifiedIDs) {
			candidates = append(candidates, patchOperationCandidate{name: "remove-reorder-add-" + moveCandidate.name, ops: ops})
		}
	}

	return candidates
}

func createRemoveThenAddThenReorderPatchCandidates(originalIDs []string, modifiedIDs []string, modified []map[string]interface{}) []patchOperationCandidate {
	originalIDSet := stringSet(originalIDs)
	currentIDs, opsWithAdds := removeIDsNotInTarget(originalIDs, stringSet(modifiedIDs))
	for targetIndex, id := range modifiedIDs {
		if originalIDSet[id] {
			continue
		}
		opsWithAdds = append(opsWithAdds, addPatchOperation(fmt.Sprintf("/%d", targetIndex), modified[targetIndex]))
		currentIDs = insertString(currentIDs, targetIndex, id)
	}

	moveCandidates := createMovePatchOperationCandidatesForIDs(currentIDs, modifiedIDs)
	candidates := make([]patchOperationCandidate, 0, len(moveCandidates))
	for _, moveCandidate := range moveCandidates {
		candidates = append(candidates, patchOperationCandidate{
			name: "remove-add-reorder-" + moveCandidate.name,
			ops:  combinePatchOperations(opsWithAdds, moveCandidate.ops),
		})
	}

	return candidates
}

func applyTargetForwardAddsAndMoves(currentIDs []string, modifiedIDs []string, modified []map[string]interface{}, ops []patchOperation) ([]string, []patchOperation, bool) {
	currentIDs = cloneStrings(currentIDs)
	ops = clonePatchOperations(ops)

	for targetIndex, id := range modifiedIDs {
		currentIndex := indexOfString(currentIDs, id)
		if currentIndex == -1 {
			ops = append(ops, addPatchOperation(fmt.Sprintf("/%d", targetIndex), modified[targetIndex]))
			currentIDs = insertString(currentIDs, targetIndex, id)
			continue
		}
		if currentIndex == targetIndex {
			continue
		}

		ops = append(ops, movePatchOperation(currentIndex, targetIndex))
		currentIDs = moveString(currentIDs, currentIndex, targetIndex)
	}

	return currentIDs, ops, true
}

func removeIDsNotInTarget(ids []string, targetIDs map[string]bool) ([]string, []patchOperation) {
	return removeIDsNotInTargetWithExistingOps(cloneStrings(ids), targetIDs, nil)
}

func removeIDsNotInTargetWithExistingOps(ids []string, targetIDs map[string]bool, ops []patchOperation) ([]string, []patchOperation) {
	ops = clonePatchOperations(ops)
	for i := len(ids) - 1; i >= 0; i-- {
		if targetIDs[ids[i]] {
			continue
		}

		ops = append(ops, removePatchOperation(fmt.Sprintf("/%d", i)))
		ids = removeString(ids, i)
	}

	return ids, ops
}

func createValuePatchOperationsAtOriginalIndexes(original []map[string]interface{}, modified []map[string]interface{}, idFieldName string) []patchOperation {
	modifiedItemsByID := mapItemsByID(modified, idFieldName)
	ops := make([]patchOperation, 0)

	for originalIndex, originalItem := range original {
		id, existsID := originalItem[idFieldName].(string)
		if !existsID {
			continue
		}

		modifiedItem, existsModifiedItem := modifiedItemsByID[id]
		if !existsModifiedItem {
			continue
		}

		ops = append(ops, createItemPatchOperations(originalItem, modifiedItem, fmt.Sprintf("/%d", originalIndex))...)
	}

	return ops
}

func createValuePatchOperationsAtTargetIndexes(original []map[string]interface{}, modified []map[string]interface{}, idFieldName string) []patchOperation {
	originalItemsByID := mapItemsByID(original, idFieldName)
	ops := make([]patchOperation, 0)

	for targetIndex, modifiedItem := range modified {
		id, existsID := modifiedItem[idFieldName].(string)
		if !existsID {
			continue
		}

		originalItem, existsOriginalItem := originalItemsByID[id]
		if !existsOriginalItem {
			continue
		}

		ops = append(ops, createItemPatchOperations(originalItem, modifiedItem, fmt.Sprintf("/%d", targetIndex))...)
	}

	return ops
}

func createItemPatchOperations(original map[string]interface{}, modified map[string]interface{}, pathPrefix string) []patchOperation {
	if reflect.DeepEqual(original, modified) {
		return nil
	}

	return jsonPatchOperationsToPatchOperations(
		PatchUsingMattbairdJsonpatchForValues(original, modified),
		pathPrefix,
	)
}

func jsonPatchOperationsToPatchOperations(jsonPatchOperations []jsonpatch.JsonPatchOperation, pathPrefix string) []patchOperation {
	ops := make([]patchOperation, 0, len(jsonPatchOperations))
	for _, jsonPatchOperation := range jsonPatchOperations {
		path := pathPrefix
		if jsonPatchOperation.Path != "" {
			path += jsonPatchOperation.Path
		}

		op := patchOperation{
			"op":   jsonPatchOperation.Operation,
			"path": path,
		}
		if jsonPatchOperation.Operation == "add" || jsonPatchOperation.Operation == "replace" || jsonPatchOperation.Operation == "test" {
			op["value"] = jsonPatchOperation.Value
		}
		ops = append(ops, op)
	}

	return ops
}

func selectShortestValidPatch(original []map[string]interface{}, modifiedJson []byte, candidates []jsonPatchCandidate) ([]byte, bool) {
	sort.SliceStable(candidates, func(i int, j int) bool {
		return len(candidates[i].patch) < len(candidates[j].patch)
	})

	for _, candidate := range candidates {
		if len(candidate.patch) == 0 || !json.Valid(candidate.patch) {
			continue
		}

		originalWithPatches, applyErr := ApplyPatch(original, candidate.patch)
		if applyErr != nil {
			continue
		}

		if !evanphxjsonpatch.Equal(originalWithPatches, modifiedJson) {
			continue
		}

		return candidate.patch, true
	}

	return nil, false
}

func appendPatchOperationCandidate(candidates *[]jsonPatchCandidate, ops []patchOperation) {
	patchBytes, err := json.Marshal(ops)
	if err != nil {
		return
	}

	*candidates = append(*candidates, jsonPatchCandidate{patch: patchBytes})
}

func dedupePatchOperationCandidates(candidates []patchOperationCandidate) []patchOperationCandidate {
	seen := make(map[string]bool, len(candidates))
	deduped := make([]patchOperationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		patchBytes, err := json.Marshal(candidate.ops)
		if err != nil {
			continue
		}
		key := string(patchBytes)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, candidate)
	}

	return deduped
}

func createMovePatchOperationCandidatesForIDs(currentIDs []string, targetIDs []string) []patchOperationCandidate {
	if !hasSameStringMembers(currentIDs, targetIDs) {
		return nil
	}

	if idsEqual(currentIDs, targetIDs) {
		return []patchOperationCandidate{{name: "already-sorted", ops: []patchOperation{}}}
	}

	candidates := make([]patchOperationCandidate, 0, 6)
	if ops, err := generateLCSMovePatchOperationsForIDs(currentIDs, targetIDs); err == nil {
		candidates = append(candidates, patchOperationCandidate{
			name: "move-lcs",
			ops:  ops,
		})
	}
	if len(currentIDs) > maxItemsForGreedyMoveCandidates {
		return dedupePatchOperationCandidates(candidates)
	}

	for _, method := range []int{1, 2, 3, 4, 5} {
		ops, err := generateMovePatchOperationsForIDs(currentIDs, targetIDs, method)
		if err != nil {
			continue
		}
		candidates = append(candidates, patchOperationCandidate{
			name: fmt.Sprintf("move-method-%d", method),
			ops:  ops,
		})
	}

	return dedupePatchOperationCandidates(candidates)
}

func generateLCSMovePatchOperationsForIDs(currentIDs []string, targetIDs []string) ([]patchOperation, error) {
	if !hasSameStringMembers(currentIDs, targetIDs) {
		return nil, fmt.Errorf("unable to generate LCS move patch for different ID sets")
	}
	if hasDuplicatedString(currentIDs) || hasDuplicatedString(targetIDs) {
		return nil, fmt.Errorf("unable to generate LCS move patch for duplicated IDs")
	}

	currentIDs = cloneStrings(currentIDs)
	keepIDs := stringSet(longestCommonSubsequence(currentIDs, targetIDs))
	targetIndexes := stringIndexes(targetIDs)
	ops := make([]patchOperation, 0, len(currentIDs)-len(keepIDs))

	for targetIndex, targetID := range targetIDs {
		if currentIDs[targetIndex] == targetID {
			continue
		}

		if keepIDs[targetID] && !keepIDs[currentIDs[targetIndex]] {
			path := targetIndexes[currentIDs[targetIndex]]
			ops = append(ops, movePatchOperation(targetIndex, path))
			currentIDs = moveString(currentIDs, targetIndex, path)
			continue
		}

		currentIndex := indexOfString(currentIDs, targetID)
		if currentIndex == -1 {
			return nil, fmt.Errorf("unable to find ID %s while generating LCS move patch", targetID)
		}

		ops = append(ops, movePatchOperation(currentIndex, targetIndex))
		currentIDs = moveString(currentIDs, currentIndex, targetIndex)
	}

	if !idsEqual(currentIDs, targetIDs) {
		return nil, fmt.Errorf("unable to generate LCS move patch")
	}

	return ops, nil
}

func longestCommonSubsequence(a []string, b []string) []string {
	if len(a) == 0 || len(b) == 0 || hasDuplicatedString(a) || hasDuplicatedString(b) {
		return nil
	}

	targetIndexes := stringIndexes(b)
	positions := make([]int, 0, len(a))
	positionIDs := make([]string, 0, len(a))
	for _, id := range a {
		targetIndex, exists := targetIndexes[id]
		if !exists {
			continue
		}
		positions = append(positions, targetIndex)
		positionIDs = append(positionIDs, id)
	}

	tailValues := make([]int, 0, len(positions))
	tailIndexes := make([]int, 0, len(positions))
	previousIndexes := make([]int, len(positions))
	for i := range previousIndexes {
		previousIndexes[i] = -1
	}

	for i, position := range positions {
		tailIndex := sort.Search(len(tailValues), func(j int) bool {
			return tailValues[j] >= position
		})
		if tailIndex > 0 {
			previousIndexes[i] = tailIndexes[tailIndex-1]
		}

		if tailIndex == len(tailValues) {
			tailValues = append(tailValues, position)
			tailIndexes = append(tailIndexes, i)
		} else {
			tailValues[tailIndex] = position
			tailIndexes[tailIndex] = i
		}
	}

	if len(tailIndexes) == 0 {
		return nil
	}

	lcs := make([]string, len(tailIndexes))
	for i, positionIndex := len(lcs)-1, tailIndexes[len(tailIndexes)-1]; i >= 0; i-- {
		lcs[i] = positionIDs[positionIndex]
		positionIndex = previousIndexes[positionIndex]
	}

	return lcs
}

func generateMovePatchOperationsForIDs(currentIDs []string, targetIDs []string, method int) ([]patchOperation, error) {
	currentIDs = cloneStrings(currentIDs)
	ops := make([]patchOperation, 0)
	maxSteps := (len(currentIDs) + len(targetIDs)) * 2
	if maxSteps < 50 {
		maxSteps = 50
	}

	for steps := 0; steps <= maxSteps; steps++ {
		from, path, changed := findNextMove(currentIDs, targetIDs, method)
		if !changed {
			if idsEqual(currentIDs, targetIDs) {
				return ops, nil
			}
			return nil, fmt.Errorf("unable to generate move patch with method %d", method)
		}

		ops = append(ops, movePatchOperation(from, path))
		currentIDs = moveString(currentIDs, from, path)
	}

	return nil, fmt.Errorf("too many patches to generate JSON patch with method %d", method)
}

func findNextMove(currentIDs []string, targetIDs []string, method int) (int, int, bool) {
	switch method {
	case 1:
		for i, currentID := range currentIDs {
			for j, targetID := range targetIDs {
				if currentID == targetID && i != j {
					return i, j, true
				}
			}
		}
	case 2:
		for i := len(currentIDs) - 1; i >= 0; i-- {
			for j := len(targetIDs) - 1; j >= 0; j-- {
				if currentIDs[i] == targetIDs[j] && i != j {
					newIndex := j
					if j > i {
						newIndex = j - 1
					}
					return i, newIndex, true
				}
			}
		}
	case 3:
		for j, targetID := range targetIDs {
			for i, currentID := range currentIDs {
				if currentID == targetID && i != j {
					return i, j, true
				}
			}
		}
	case 4:
		for j := len(targetIDs) - 1; j >= 0; j-- {
			for i, currentID := range currentIDs {
				if currentID == targetIDs[j] && i != j {
					return i, j, true
				}
			}
		}
	case 5:
		for i := len(currentIDs) - 1; i >= 0; i-- {
			for j := len(targetIDs) - 1; j >= 0; j-- {
				if currentIDs[i] == targetIDs[j] && i != j {
					return i, j, true
				}
			}
		}
	}

	return 0, 0, false
}

func addPatchOperation(path string, value interface{}) patchOperation {
	return patchOperation{
		"op":    "add",
		"path":  path,
		"value": value,
	}
}

func removePatchOperation(path string) patchOperation {
	return patchOperation{
		"op":   "remove",
		"path": path,
	}
}

func movePatchOperation(from int, path int) patchOperation {
	return patchOperation{
		"op":   "move",
		"from": fmt.Sprintf("/%d", from),
		"path": fmt.Sprintf("/%d", path),
	}
}

func idsFromItems(items []map[string]interface{}, idFieldName string) ([]string, bool) {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		id, existsID := item[idFieldName].(string)
		if !existsID {
			return nil, false
		}
		ids = append(ids, id)
	}

	return ids, true
}

func mapItemsByID(items []map[string]interface{}, idFieldName string) map[string]map[string]interface{} {
	itemsByID := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		id, existsID := item[idFieldName].(string)
		if !existsID {
			continue
		}
		itemsByID[id] = item
	}

	return itemsByID
}

func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}

	return set
}

func stringIndexes(items []string) map[string]int {
	indexes := make(map[string]int, len(items))
	for i, item := range items {
		indexes[item] = i
	}

	return indexes
}

func hasDuplicatedString(items []string) bool {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if seen[item] {
			return true
		}
		seen[item] = true
	}

	return false
}

func hasSameStringMembers(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[string]int, len(a))
	for _, item := range a {
		counts[item]++
	}
	for _, item := range b {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}

	return true
}

func idsEqual(a []string, b []string) bool {
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

func indexOfString(items []string, value string) int {
	for i, item := range items {
		if item == value {
			return i
		}
	}

	return -1
}

func insertString(items []string, index int, value string) []string {
	if index >= len(items) {
		return append(items, value)
	}

	items = append(items, "")
	copy(items[index+1:], items[index:])
	items[index] = value
	return items
}

func removeString(items []string, index int) []string {
	return append(items[:index], items[index+1:]...)
}

func moveString(items []string, from int, path int) []string {
	value := items[from]
	items = removeString(items, from)
	return insertString(items, path, value)
}

func cloneStrings(items []string) []string {
	cloned := make([]string, len(items))
	copy(cloned, items)
	return cloned
}

func clonePatchOperations(ops []patchOperation) []patchOperation {
	if len(ops) == 0 {
		return nil
	}

	cloned := make([]patchOperation, len(ops))
	copy(cloned, ops)
	return cloned
}

func combinePatchOperations(a []patchOperation, b []patchOperation) []patchOperation {
	combined := make([]patchOperation, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)
	return combined
}

func CreateMovePatches(arr1 []byte, arr2 []byte, idFieldName string) ([]byte, error) {
	arr1AsMap := GetMapFromByte(arr1)
	arr2AsMap := GetMapFromByte(arr2)
	currentIDs, currentIDsOk := idsFromItems(arr1AsMap, idFieldName)
	targetIDs, targetIDsOk := idsFromItems(arr2AsMap, idFieldName)
	if !currentIDsOk || !targetIDsOk {
		return nil, fmt.Errorf("unable to generate move patch without valid %s fields", idFieldName)
	}

	moveCandidates := createMovePatchOperationCandidatesForIDs(currentIDs, targetIDs)
	if len(moveCandidates) == 0 {
		return nil, fmt.Errorf("unable to generate move patch")
	}

	var bestPatch []byte
	for _, moveCandidate := range moveCandidates {
		patchBytes, err := json.Marshal(moveCandidate.ops)
		if err != nil {
			continue
		}
		if bestPatch == nil || len(patchBytes) < len(bestPatch) {
			bestPatch = patchBytes
		}
	}
	if bestPatch == nil {
		return nil, fmt.Errorf("unable to marshal move patch")
	}

	return bestPatch, nil
}

func ApplyPatch(original []map[string]interface{}, patchBytes []byte) ([]byte, error) {
	originalBytes, _ := json.Marshal(original)
	patch, err := evanphxjsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, err
	}

	modifiedJson, _ := patch.Apply(originalBytes)

	return modifiedJson, nil
}

func PatchUsingMattbairdJsonpatch(original []map[string]interface{}, modified []map[string]interface{}) []jsonpatch.JsonPatchOperation {
	return PatchUsingMattbairdJsonpatchForValues(original, modified)
}

func PatchUsingMattbairdJsonpatchForValues(original interface{}, modified interface{}) []jsonpatch.JsonPatchOperation {
	oldListJson, _ := json.Marshal(original)
	newListJson, _ := json.Marshal(modified)

	patches, _ := jsonpatch.CreatePatch(oldListJson, newListJson)

	return patches
}

func GetMapFromByte(jsonAsByte []byte) []map[string]interface{} {
	var jsonAsMap []map[string]interface{}
	if len(jsonAsByte) > 0 {
		err := json.Unmarshal(jsonAsByte, &jsonAsMap)
		if err != nil {
			log.Debug(jsonAsByte)
			log.Error("Error Unmarshal GetMapFromByte:", err)
			return nil
		}
	}

	return jsonAsMap
}

func MergePatches(patchA []byte, patchB []byte) ([]byte, error) {
	patchAMap := GetMapFromByte(patchA)
	patchBMap := GetMapFromByte(patchB)

	mergedOps := append(patchAMap, patchBMap...)
	mergedJSON, err := json.Marshal(mergedOps)
	if err != nil {
		return nil, err
	}

	return mergedJSON, nil
}

func findItemWithId(itemMaps []map[string]interface{}, id string, defaultValue map[string]interface{}, idFieldName string) map[string]interface{} {
	for _, u := range itemMaps {
		if idField, existsIdField := u[idFieldName].(string); existsIdField {
			if idField == id {
				return u
			}
		}
	}

	return defaultValue
}

func PrintJsonPatchOperation(it []jsonpatch.JsonPatchOperation, name string) {
	fmt.Printf("%s:\n", name)
	a, _ := json.Marshal(it)
	PrintJson(a, name)
}

func PrintMap(it []map[string]interface{}, name string) {
	fmt.Printf("%s:\n", name)
	a, _ := json.Marshal(it)
	PrintJson(a, name)
}

func PrintJson(it []byte, name string) {
	fmt.Printf("%s:\n", name)
	fmt.Println(string(it))
}
