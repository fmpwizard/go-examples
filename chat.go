package main

import (
	"encoding/json"
	"github.com/emicklei/go-restful"
	"github.com/nu7hatch/gouuid"
	"io/ioutil"
	"math/rand"
	"sync"

	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"time"
)

//messageStore 's key is sessionId + cometId'
var messageStore = struct {
	sync.RWMutex
	LastIndex uint64
	m         map[sessionCometKey][]message
}{m: make(map[sessionCometKey][]message)}

//cometStore 's key is sessionId'
var cometStore = struct {
	sync.RWMutex
	m map[session]comet
}{m: make(map[session]comet)}

type message struct {
	index uint64
	Value jsCmd `json:"value"`
	Stamp time.Time
}

type comet struct {
	Value    string
	LastSeen time.Time
}

type CometInfo struct {
	CometId string
	Index   uint64
}

type Response struct {
	Value jsCmd
	Error string
}

type Responses struct {
	Res       []Response
	LastIndex uint64
}

func (r Responses) Encode() []byte {
	b, err := json.Marshal(r)
	if err != nil {
		return []byte("")
	}
	return b
}

type sessionCometKey string

type session string

type jsCmd struct {
	Js string `json:"js"`
}

///////////////////

type Message struct {
	Id        string `json:"id"`
	Body      string `json:"body"`
	CreatedOn int64  `json:"createdOn"`
}

type CometResponse struct {
	Event string  `json:"event"`
	Data  Message `json:"data"`
}

type ChatMessageResource struct {
	// normally one would use DAO (data access object)
	messages map[string]Message
}

type MessageStore struct {
	chatMessages *ChatMessageResource
	msg          Message
}

type ByCreatedOn []Message

func (m ByCreatedOn) Len() int {
	return len(m)
}

func (m ByCreatedOn) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m ByCreatedOn) Less(i, j int) bool {
	return m[i].CreatedOn < m[j].CreatedOn
}

var rootDir string

func init() {
	currentDir, _ := os.Getwd()
	flag.StringVar(&rootDir, "root-dir", currentDir, "specifies the root dir where html and other files will be relative to")
}

var messages = ChatMessageResource{map[string]Message{}}

var lpchan = make(chan chan Message)

func main() {
	flag.Parse()
	http.HandleFunc("/index", showMessages)
	http.HandleFunc("/api/messages/new", createChatMessage)
	http.Handle("/bower_components/", http.StripPrefix("/bower_components/", http.FileServer(http.Dir("app/bower_components"))))
	http.Handle("/build/", http.StripPrefix("/build/", http.FileServer(http.Dir("build"))))
	log.Println("Listening ...")
	log.Fatal(http.ListenAndServe(":7070", nil))
}

// Register tells go-restful about our API uri's
func (chatMessages *ChatMessageResource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/api").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.Route(ws.GET("/messages/{message-id}").To(chatMessages.retrieveChatMessage))
	ws.Route(ws.GET("/messages/page/{last-page}").To(chatMessages.retrieveChatMessages))
	ws.Route(ws.GET("/comet/{session-id}/{page-id}").To(chatMessages.handleComet))
	container.Add(ws)

}

func createChatMessage(rw http.ResponseWriter, req *http.Request) {

	guid, err := uuid.NewV4()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	data := Message{Id: guid.String()}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("Error reading Body, got %v", err)
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("4 error ", err)
	}

	ret := "console.log('" + data.Body + "');"
	currentComet := req.FormValue("cometid") //TODO make sure to pass this in
	cookie, _ := req.Cookie("gsessionid")
	messageStore.Lock()
	messageStore.LastIndex++
	messageStore.m[sessionCometKey(cookie.Value+currentComet)] = append(messageStore.m[sessionCometKey(cookie.Value+currentComet)], message{messageStore.LastIndex, jsCmd{ret}, time.Now()})
	messageStore.Unlock()

	jsonRet, err := json.Marshal(map[string]string{"id": guid.String()})
	if err != nil {
		fmt.Printf("Error marshalling %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Header().Add("Content-Type", "text/plain")
	} else {
		rw.WriteHeader(http.StatusCreated)
		rw.Write(jsonRet)

	}

}

/*func createChatMessage(rw http.ResponseWriter, req *http.Request) {
	//TODO
	guid, err := uuid.NewV4()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	msg := Message{Id: guid.String()}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("Error reading Body, got %v", err)
	}
	err = json.Unmarshal(body, &msg)
	if err != nil {
		fmt.Println("4 error ", err)
	}

	if parseErr == nil {
		fmt.Println("4")
	Loop:
		for {
			select {
			case clientchan := <-lpchan:
				fmt.Println("5")
				clientchan <- msg
			default:
				break Loop
			}
		}

		ret := map[string]string{"id": guid.String()}

		response.WriteHeader(http.StatusCreated)
		response.WriteEntity(ret)
	} else {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, parseErr.Error())
	}
}*/

func (chatMessages *ChatMessageResource) retrieveChatMessages(request *restful.Request, response *restful.Response) {
	lastPage, err := strconv.ParseInt(request.PathParameter("last-page"), 10, 0)
	if err != nil {
		fmt.Errorf("Count not format page number to int", err)
	}
	//fmt.Printf("last page is: %s\n", lastPage)
	ret := sortMessages(chatMessages, lastPage)
	response.WriteEntity(ret)
}

func sortMessages(msgs *ChatMessageResource, page int64) ByCreatedOn {
	s := make(ByCreatedOn, 0, len(msgs.messages))
	for _, d := range msgs.messages {
		s = append(s, d)
	}
	sort.Sort(sort.Reverse(ByCreatedOn(s)))
	return paginate(s, page)
}

func paginate(data []Message, page int64) []Message {
	pageSize := 10
	skip := int(page) * pageSize
	if skip > len(data) {
		skip = len(data)
	}

	end := skip + pageSize
	if end > len(data) {
		end = len(data)
	}

	ret := data[skip:end]
	sort.Sort(ByCreatedOn(ret))

	return ret
}

func showMessages(rw http.ResponseWriter, req *http.Request) {
	t := template.New("messages.html")
	t.Funcs(template.FuncMap{"UnixToString": UnixToString})
	t, err := t.ParseFiles(path.Join(rootDir, "app/messages.html"))
	if err != nil {
		fmt.Printf("Error parsing template files: %v", err)
	}
	cookie, err := req.Cookie("gsessionid")
	if err == http.ErrNoCookie {
		rand.Seed(time.Now().UnixNano())
		sess := strconv.FormatFloat(rand.Float64(), 'f', 20, 64)
		cookie = &http.Cookie{
			Name:    "gsessionid",
			Value:   sess,
			Path:    "/",
			Expires: time.Now().Add(60 * time.Hour),
		}
		http.SetCookie(rw, cookie)
	}
	var cometId string
	var index uint64
	rw.Header().Add("Content-Type", "text/html; charset=UTF-8")
	cometStore.RLock()
	cometVal, found := cometStore.m[session(cookie.Value)]
	cometStore.RUnlock()
	if found {
		cometId = cometVal.Value
	} else {
		//create comet for the first time
		rand.Seed(time.Now().UnixNano())
		cometId = strconv.FormatFloat(rand.Float64(), 'f', 20, 64)
		cometStore.Lock()
		cometStore.m[session(cookie.Value)] = comet{cometId, time.Now()}
		cometStore.Unlock()
	}

	messageStore.RLock()
	_, found = messageStore.m[sessionCometKey(cookie.Value+cometId)]
	lastId := messageStore.LastIndex
	messageStore.RUnlock()
	if found {
		index = lastId
	}

	err = t.ExecuteTemplate(rw, "messages.html", CometInfo{cometId, index})
	if err != nil {
		log.Fatalf("got error: %s", err)
	}

}

/*func showMessages(rw http.ResponseWriter, req *http.Request) {
	ret := sortMessages(&messages, 0)
	t := template.New("messages.html")
	t.Funcs(template.FuncMap{"UnixToString": UnixToString})
	t, err := t.ParseFiles(path.Join(rootDir, "app/messages.html"))
	if err != nil {
		panic(err)
	}
	rw.Header().Add("Content-Type", "text/html; charset=UTF-8")
	err = t.ExecuteTemplate(rw, "messages.html", ret)
	if err != nil {
		panic(err)
	}

}

*/func UnixToString(x int64) string {
	ret := time.Unix(x/1000, 0)
	return ret.String()
}

func (chatMessages *ChatMessageResource) retrieveChatMessage(request *restful.Request, response *restful.Response) {
	messageId := request.PathParameter("message-id")
	msg, found := chatMessages.messages[messageId]
	if found {
		response.WriteEntity(msg)
	} else {
		response.WriteErrorString(http.StatusNotFound, "Message not found")
	}

}

func serveResources(req *restful.Request, resp *restful.Response) {
	uno := req.PathParameter("uno")

	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		path.Join(rootDir, "build/", uno))
}

func (chatMessages *ChatMessageResource) handleComet(request *restful.Request, response *restful.Response) {
	//sessionId := request.PathParameter("session-id")
	//pageId := request.PathParameter("page-id")

	//var ret CometResponse

	fmt.Println("0")
	myRequestChan := make(chan Message)

	select {
	case lpchan <- myRequestChan:
		//ret = CometResponse{"dataMessageSaved", m}
		fmt.Println("1")
	case <-time.After(7 * time.Second):
		fmt.Println("2")
		return
	}

	ret := <-myRequestChan

	fmt.Printf("3 %v\n", ret)
	response.WriteEntity(CometResponse{"dataMessageSaved", ret})
}
