package msgpatch

import (
	"encoding/json"
	"github.com/iMDT/bbb-graphql-middleware/internal/common"
	"github.com/mattbaird/jsonpatch"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

var cacheDir = os.TempDir() + "/graphql-middleware-cache/"
var minLengthToPatch = 250    //250 chars
var minShrinkToUsePatch = 0.5 //50% percent

func getConnPath(connectionId string) string {
	return cacheDir + connectionId
}

func getSubscriptionCacheDirPath(bConn *common.BrowserConnection, subscriptionId string, createIfNotExists bool) (string, error) {
	//Using SessionToken as path to reinforce security (once connectionId repeats on restart of middleware)
	connectionPatchCachePath := getConnPath(bConn.Id) + "/" + bConn.SessionToken + "/"
	subscriptionCacheDirPath := connectionPatchCachePath + subscriptionId + "/"
	_, err := os.Stat(subscriptionCacheDirPath)
	if err != nil {
		if os.IsNotExist(err) && createIfNotExists {
			err = os.MkdirAll(subscriptionCacheDirPath, 0755)
			if err != nil {
				log.Errorf("Error on create cache directory:", err)
				return subscriptionCacheDirPath, nil
			}
		} else {
			return "", err
		}
	}

	return subscriptionCacheDirPath, nil
}

func RemoveConnCacheDir(connectionId string) {
	err := os.RemoveAll(getConnPath(connectionId))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("Error while removing CLI patch cache directory:", err)
		}
		return
	}

	log.Debugf("Directory of patch caches removed successfully for client %s.", connectionId)
}

func RemoveConnSubscriptionCacheFile(bConn *common.BrowserConnection, subscriptionId string) {
	subsCacheDirPath, err := getSubscriptionCacheDirPath(bConn, subscriptionId, false)
	if err == nil {
		err = os.RemoveAll(subsCacheDirPath)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Errorf("Error while removing CLI subscription patch cache directory:", err)
			}
			return
		}

		log.Debugf("Directory of patch caches removed successfully for client %s, subscription %s.", bConn.Id, subscriptionId)
	}
}

func ClearAllCaches() {
	info, err := os.Stat(cacheDir)
	if err == nil && info.IsDir() {
		filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Debugf("Cache dir was removed previously (probably user disconnected): %q: %v\n", path, err)
				return err
			}

			if info.IsDir() && path != cacheDir {
				os.RemoveAll(path)
			}
			return nil
		})
	}
}

func PatchMessage(receivedMessage *map[string]interface{}, queryId string, dataKey string, dataAsJson []byte, bConn *common.BrowserConnection, cacheKey string) {
	var receivedMessageMap = *receivedMessage

	fileCacheDirPath, err := getSubscriptionCacheDirPath(bConn, queryId, true)
	if err != nil {
		log.Errorf("Error on get Client/Subscription cache path: %v", err)
		return
	}
	filePath := fileCacheDirPath + dataKey + ".json"

	lastContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		//Last content doesn't exist, probably it's the first response
	}
	lastDataAsJsonString := string(lastContent)
	if string(dataAsJson) == lastDataAsJsonString {
		//Content didn't change, set message as null to avoid sending it to the browser
		//This case is usual when the middleware reconnects with Hasura and receives the data again
		*receivedMessage = nil
	} else {
		//Content was changed, creating json patch
		//If data is small (< minLengthToPatch) it's not worth creating the patch
		if lastDataAsJsonString != "" && len(string(dataAsJson)) > minLengthToPatch {

			if _, exists := common.JsonPatchCache[cacheKey]; !exists {
				diffPatch, e := jsonpatch.CreatePatch([]byte(lastDataAsJsonString), []byte(dataAsJson))
				if e != nil {
					log.Errorf("Error creating JSON patch:%v", e)
					return
				}
				jsonDiffPatch, err := json.Marshal(diffPatch)
				if err != nil {
					log.Errorf("Error marshaling patch array:", err)
					return
				}

				common.JsonPatchCache[cacheKey] = jsonDiffPatch
			}

			//Use patch if the length is {minShrinkToUsePatch}% smaller than the original msg
			if float64(len(string(common.JsonPatchCache[cacheKey])))/float64(len(string(dataAsJson))) < minShrinkToUsePatch {
				//Modify receivedMessage to include the Patch and remove the previous data
				//The key of the original message is kept to avoid errors (Apollo-client expects to receive this prop)
				receivedMessageMap["payload"] = map[string]interface{}{
					"data": map[string]interface{}{
						"patch": json.RawMessage(common.JsonPatchCache[cacheKey]),
						dataKey: json.RawMessage("[]"),
					},
				}
				*receivedMessage = receivedMessageMap
			}
		}

		//Store current result to be used to create json patch in the future
		if lastDataAsJsonString != "" || len(string(dataAsJson)) > minLengthToPatch {
			errWritingOutput := ioutil.WriteFile(filePath, []byte(dataAsJson), 0644)
			if errWritingOutput != nil {
				log.Errorf("Error on trying to write cache of json diff:", errWritingOutput)
			}
		}
	}
}
