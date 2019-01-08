package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

//
// SpecField contains spec fields that should be updated in crd
//
type SpecField struct {
    CPUPCT *string `json:"cpuPCT"`
    MemPCT *string `json:"memPCT"`
}

//
// Spec is For JSON format
//
type Spec struct {
    Field SpecField `json:"spec"`
}

func StringPtr(s string) *string {
    return &s
}

//
// MergePatchTest make a HTTP Patch request to update the app corresponding to the appName
//
func MergePatchTest(url string, appName string, specField SpecField) {
    content := Spec{
        specField,
    }
    data, _ := json.Marshal(content)
    fmt.Printf("%s\n", data)

    // url := "http://localhost:8001/apis/app.example.com/v1alpha1/namespaces/default/myapps/test-app1"
    // Make HTTP Patch request
    client := &http.Client{}
    req, err := http.NewRequest(http.MethodPatch, url+appName, bytes.NewReader(data))
    if err != nil {
        panic(err)
    }

    // Set header
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/merge-patch+json")
    req.Header.Set("User-Agent", "kubectl/v1.10.0 (linux/amd64) kubernetes/fc32d2f")

    res, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer res.Body.Close()
}
