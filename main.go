package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

type Configuration struct {
	Title      string
	Secretkey  string
	MinSymbols int
	Threads    int
}

var config = Configuration{
	Title:      "zimbra-mail-cleaner",
	Secretkey:  "Secretkey",
	MinSymbols: 4,
	Threads:    2,
}

var mutex = &sync.Mutex{}

const mutexLocked = 1

func MutexLocked(m *sync.Mutex) bool {
	state := reflect.ValueOf(m).Elem().FieldByName("state")
	return state.Int()&mutexLocked == mutexLocked
}

func runningPS() []byte {
	cmdPart := "ps -aux | grep '[Z]MailboxUtil'"
	cmd := exec.Command("bash", "-c", cmdPart)
	out, _ := cmd.CombinedOutput()
	// if err != nil {
	// fmt.Fatalf("cmd.Run() failed with %s\n", err)
	// }
	return out
}

func spawnCmd(subject string) {
	mutex.Lock()
	allmails := getAllUsers()
	allmails2 := splitAllUsers(allmails)
	for _, mailList := range allmails2 {
		go searchUserMessages(mailList, subject)
	}
	mutex.Unlock()
}

func getAllUsers() []string {
	cmd := exec.Command("zmprov", "-l", "gaa")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("env ERROR? %s", err)
		return nil
	}
	q := strings.Split(string(out), "\n")
	RemoveEmpty(&q)
	return q
}

func splitAllUsers(s []string) [][]string {
	var ss [][]string
	for i := range config.Threads {
		ss = append(ss, s[i*len(s)/config.Threads:(i+1)*len(s)/config.Threads])
	}
	return ss
}

func searchUserMessages(mailList []string, subject string) {
	for _, mail := range mailList {
		fmt.Printf("Searching: %s (%s) \n", mail, subject)
		cmdPart := fmt.Sprintf("zmmailbox -z -m %s s -l 999 -t message \"subject: %s\"", mail, subject)
		cmd := exec.Command("bash", "-c", cmdPart)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("zmmailbox ERROR:  %s \n", err)
		}
		var matchList []string
		for _, line := range strings.Split(string(out), "\n") {
			re := regexp.MustCompile(`(^\d\W\W(?P<Id>\d+)\W+mess)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 0 {
				Id := re.SubexpIndex("Id")
				matchList = append(matchList, matches[Id])
			}
		}
		if len(matchList) > 0 {
			fmt.Println(matchList)
		}
	}
}

func RemoveEmpty(slice *[]string) {
	i := 0
	p := *slice
	for _, entry := range p {
		if strings.Trim(entry, " ") != "" {
			p[i] = entry
			i++
		}
	}
	*slice = p[0:i]
}

func main() {
	// Handle POST and GET requests.
	// fmt.SetFlags(fmt.LstdFlags | fmt.Lmicroseconds)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()
			fmt.Println("POST METHOD")
			if len(r.FormValue("secretkey")) != 0 && len(r.FormValue("subject")) != 0 {
				if r.FormValue("secretkey") == config.Secretkey && len(r.FormValue("subject")) >= config.MinSymbols {
					processing := string(runningPS())
					if processing == "" && !MutexLocked(mutex) {
						fmt.Printf("Form input %s, %s \n", r.FormValue("secretkey"), r.FormValue("subject"))
						go spawnCmd(r.FormValue("subject"))
						component := page(config.Title, "", true)
						component.Render(r.Context(), w)
					} else {
						component := page(config.Title, processing, true)
						component.Render(r.Context(), w)
					}
				} else {
					fmt.Printf("%#v %#v", r.FormValue("secretkey"), r.FormValue("subject"))
					component := errorPage(config.Title, "AuthError")
					component.Render(r.Context(), w)
				}
			} else {
				component := errorPage(config.Title, "ParamError")
				component.Render(r.Context(), w)
			}
			return
		}
		fmt.Println("GET METHOD")
		component := page(config.Title, string(runningPS()), false)
		component.Render(r.Context(), w)
	})
	fmt.Println("listening on http://localhost:8001")
	if err := http.ListenAndServe("0.0.0.0:8001", nil); err != nil {
		fmt.Printf("error listening: %v", err)
	}
}
