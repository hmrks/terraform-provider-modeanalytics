package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// CheckDeletion verifies the deletion of a resource by attempting to read it every 10 seconds for a minute.
// It returns an error if the status code is anything other than 200 or 404.
func CheckDeletion(resourceURL string, client *http.Client) error {
	timeout := time.After(1 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("deletion verification timed out after 1 minute")
		case <-ticker.C:
			resp, err := client.Get(resourceURL)
			if err != nil {
				return fmt.Errorf("error making GET request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Println("Deletion verified: resource not found (404)")
				return nil
			} else if resp.StatusCode == http.StatusForbidden {
				// There is a bug where a GET request on a freshly deleted collection returns 403 instead of 404.
				// So as a workaround, we list all collections. If we have the correct access rights to do so,
				// we assume everything is alright.
				matched, _ := regexp.MatchString("^https://[^/]+/api/[^/]+/spaces/[^/]+$", resourceURL)
				if matched {
					url := strings.Split(resourceURL, "/spaces/")[0] + "/spaces?filter=all"
					httpReq, err := http.NewRequest("GET", url, nil)
					if err != nil {
						return fmt.Errorf("Unable to read collections during deletion verification, got error: %s", err)
					}

					httpResp, err := client.Do(httpReq)
					if err != nil {
						return fmt.Errorf("Unable to read collections during deletion verification, got error: %s", err)
					}
					defer httpResp.Body.Close()

					if httpResp.StatusCode != http.StatusOK {
						return fmt.Errorf("Unable to read collections during deletion verification, got error: %s", err)
					} else {
						return nil
					}
				} else {
					return fmt.Errorf("unexpected status: %d, not retrying", resp.StatusCode)
				}
			} else if resp.StatusCode == http.StatusOK {
				var responseData struct {
					State string `json:"state"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
					return fmt.Errorf("error decoding response: %v", err)
				}

				if responseData.State == "soft_deleted" {
					fmt.Println("Deletion verified: resource is soft deleted")
					return nil
				} else {
					fmt.Println("Resource state:", responseData.State, "still active, retrying...")
					continue // Continue retrying as long as the status is 200 and not soft_deleted
				}
			} else {
				return fmt.Errorf("unexpected status or state: %d, not retrying", resp.StatusCode)
			}
		}
	}
}

func HttpRetry(ctx context.Context, method, url string, body io.Reader) (*Request, error) {
	sleep := 10 * time.Second
	attempts := 9

	for i := 0; i < attempts; i++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if httpReq.StatusCode != http.StatusTooManyRequests {
			break
		}
		time.Sleep(sleep)
	}
	return httpReq, err
}