package main

import (
    "encoding/json"
    "fmt"
    "github.com/shirou/gopsutil/process"
    "io/ioutil"
    "net/http"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "syscall"
    "time"
)

// App contains deployed apps' info
type App struct {
    name      string
    pid       int
    shellFile string
}

type AppSpec struct {
    shellFile string
}

// func main() {
//     MergePatchTest()
// }

func main() {

    apps := make(map[string]App)
    // url := "http://localhost:8001/apis/app.example.com/v1alpha1/namespaces/default/ncapps/"
    url := "http://localhost:8001/apis/app.example.com/v1alpha1/ncapps/"

    for {

        hostname := getHostName()
        fmt.Println("hostname: ", hostname)

        // Url "app.example.com" must match the config spec.group in myapp-crd.yaml
        // URL "myapps" must match the config spec.names.plural in myapp-crd.yaml
        resp, err := http.Get(url)
        if err != nil {
            panic(err)
        }
        var data map[string]interface{}
        respBody, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            panic(err)
        }
        err = json.Unmarshal([]byte(respBody), &data)
        if err != nil {
            panic(err)
        }

        items := data["items"].([]interface{})
        appNames := make(map[string]AppSpec, 0)
        for i := range items {
            item := items[i].(map[string]interface{})

            metadata := item["metadata"].(map[string]interface{})
            appName := metadata["name"].(string)
            fmt.Println("item: ", appName)

            spec := item["spec"].(map[string]interface{})
            nodeName := spec["nodeName"].(string)
            fmt.Println("nodeName: ", nodeName)
            shellFile := spec["shellFile"].(string)
            fmt.Println("shellFile: ", shellFile)
            // if the nodeName in spec is equal to hostname, then add the appName into the appNames
            if strings.Compare(hostname, nodeName) == 0 {
                appNames[appName] = AppSpec{
                    shellFile: shellFile,
                }
            }
        }

        // check the app objects deleted by k8s, kill the corresponding processes in the node
        for name := range apps {
            fmt.Printf("apps: %s\n", name)
            if _, ok := appNames[name]; !ok {
                pid := apps[name].pid
                killProcessByPID(pid)
                delete(apps, name)
            }
        }

        // check the app objects added by k8s, start the corresponding processes in the node
        // and add app's info in apps
        for name := range appNames {
            fmt.Printf("appNames: %s\n", name)
            if _, ok := apps[name]; !ok {
                // tomcatPath := "/usr/local/tomcat/apache-tomcat-8.5.35/bin/"
                // cmd := exec.Command("sh", tomcatPath+"startup.sh")
                cmd := exec.Command("sh", appNames[name].shellFile)
                err = cmd.Run() // it must be Run() rather than Start() here!!! Start() does not wait for it to complete!!!
                if err != nil {
                    panic(err)
                }

                pid := getPIDByName("tomcat")
                fmt.Printf("Start tomcat: %d\n", pid)

                apps[name] = App{
                    name:      name,
                    pid:       pid,
                    shellFile: appNames[name].shellFile,
                }
            }
        }

        for _, v := range apps {
            fmt.Printf("app: %s\n", v.name)
            fmt.Printf("Get info by %d\n", v.pid)
            p, err := process.NewProcess((int32)(v.pid))
            if err != nil {
                panic(err)
            }
            cpuPCT, err := p.CPUPercent()
            if err != nil {
                panic(err)
            }
            memPCT, err := p.MemoryPercent()
            if err != nil {
                panic(err)
            }

            fmt.Printf("CPU Percent: %f\n", cpuPCT)
            fmt.Printf("Mem Percent: %f\n", memPCT)

            specField := SpecField{
                CPUPCT: StringPtr(fmt.Sprint(cpuPCT)),
                MemPCT: StringPtr(fmt.Sprint(memPCT)),
            }
            MergePatchTest(url, v.name, specField)
        }

        time.Sleep(10 * time.Second)

        // pid := getPIDByName("tomcat")
        // killProcessByPID(pid)

        // break
    }

}

func getHostName() string {
    hostname, err := os.Hostname()
    if err != nil {
        panic(err)
    }
    return hostname
}

func getPIDByName(name string) int {
    cmd := exec.Command("pgrep", "-f", name)
    // cmd = exec.Command("sh", "-c", `pgrep -f tomcat >> tmp`)
    // cmd = exec.Command("sh", "-c", `ps axf | grep tomcat | grep -v grep | awk '{print $1}'`)
    out, err := cmd.Output()
    if err != nil {
        panic(err)
    }

    pid, err := strconv.Atoi(strings.Trim(string(out), "\n"))
    if err != nil {
        panic(err)
    }

    return pid
}

func killProcessByPID(pid int) {
    fmt.Printf("Kill process: %d\n", pid)
    err := syscall.Kill(pid, syscall.SIGTERM)
    if err != nil {
        panic(err)
    }
}
