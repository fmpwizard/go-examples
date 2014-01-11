package main

import (
	"github.com/emicklei/go-restful"
	"github.com/nu7hatch/gouuid"

	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Message struct {
	Id        string `json:"id"`
	Body      string `json:"body"`
	CreatedOn int64  `json:"createdOn"`
}

type CometResponse struct {
	Event  string  `json:"event"`
	PageId string  `json:"pageId"`
	Data   Message `json:"data"`
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
	flag.StringVar(&rootDir, "root-dir", "/home/diego/work/golang/groupchat", "specifies the root dir where html and other files will be relative to")
}

var messages = ChatMessageResource{map[string]Message{}}
var messagesChan = make(chan *MessageStore)
var cometChannel = make(chan Message)
var comets = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

func main() {
	flag.Parse()
	go handleAddMessage(messagesChan)
	staticWS := initStatic()
	wsContainer := restful.NewContainer()
	wsContainer.Add(staticWS).EnableContentEncoding(true)
	messages.Register(wsContainer)
	log.Println("Listening ...")
	log.Fatal(http.ListenAndServe(":7070", wsContainer))

}

// initStatic sets up the routes to server the index and messages page, as
// well as our css and js files
func initStatic() *restful.WebService {
	staticWS := new(restful.WebService)
	staticWS.Route(staticWS.GET("/index").To(serveIndex))
	staticWS.Route(staticWS.GET("/messages").To(showMessages))
	staticWS.Route(staticWS.GET("/bower_components/{uno}/{dos}").To(serveBowerFiles))
	staticWS.Route(staticWS.GET("/build/{uno}").To(serveResources))

	return staticWS
}

// Register tells go-restful about our API uri's
func (chatMessages *ChatMessageResource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/api").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.Route(ws.PUT("/messages/new").To(chatMessages.createChatMessage))
	ws.Route(ws.GET("/messages/{message-id}").To(chatMessages.retrieveChatMessage))
	ws.Route(ws.GET("/messages/page/{last-page}").To(chatMessages.retrieveChatMessages))
	ws.Route(ws.GET("/comet/{id}").To(chatMessages.handleComet))
	container.Add(ws)

}

// handleAddMessage reads the payload channel and adds a new entry to
// the chat messages slice as they become available.
func handleAddMessage(payload chan *MessageStore) {
	for msg := range payload {
		msg.chatMessages.messages[msg.msg.Id] = msg.msg
		cometChannel <- msg.msg
	}
}

func (chatMessages *ChatMessageResource) createChatMessage(request *restful.Request, response *restful.Response) {
	guid, err := uuid.NewV4()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	msg := Message{Id: guid.String()}
	parseErr := request.ReadEntity(&msg)
	if parseErr == nil {
		fmt.Printf("full map %v\n", comets.m)
		comets.RLock()
		fmt.Println("Got read lock")
		for key, _ := range comets.m {
			fmt.Printf("Sending message to comet id: %v\n", key)
			go func() {
				messagesChan <- &MessageStore{chatMessages, msg}
			}()
		}
		comets.RUnlock()

		ret := map[string]string{"id": guid.String()}

		response.WriteHeader(http.StatusCreated)
		response.WriteEntity(ret)
	} else {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, parseErr.Error())
	}
}

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

func showMessages(req *restful.Request, resp *restful.Response) {
	ret := sortMessages(&messages, 0)
	t := template.New("messages.html")
	t.Funcs(template.FuncMap{"UnixToString": UnixToString})
	t, err := t.ParseFiles(path.Join(rootDir, "app/messages.html"))
	if err != nil {
		panic(err)
	}
	resp.ResponseWriter.Header().Add("Content-Type", "text/html; charset=UTF-8")
	err = t.ExecuteTemplate(resp.ResponseWriter, "messages.html", ret)
	if err != nil {
		panic(err)
	}

}

func UnixToString(x int64) string {
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

func serveIndex(req *restful.Request, resp *restful.Response) {
	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		path.Join(rootDir, "app/index.html"))
}

func serveBowerFiles(req *restful.Request, resp *restful.Response) {
	uno := req.PathParameter("uno")
	dos := req.PathParameter("dos")
	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		path.Join(rootDir, "app/bower_components", uno, dos))
}

func serveResources(req *restful.Request, resp *restful.Response) {
	uno := req.PathParameter("uno")

	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		path.Join(rootDir, "build/", uno))
}

func (chatMessages *ChatMessageResource) handleComet(request *restful.Request, response *restful.Response) {

	cometId := request.PathParameter("id")
	fmt.Printf("Responding to comet id %v\n", cometId)

	comets.Lock()
	comets.m[cometId] = "msg.Id"
	comets.Unlock()

	for key, value := range comets.m {
		fmt.Printf("key is %v and value is %v\n", key, value)
	}

	var ret CometResponse

	select {
	case m := <-cometChannel:
		ret = CometResponse{"dataMessageSaved", cometId, m}
		comets.Lock()
		delete(comets.m, cometId)
		comets.Unlock()

	case <-time.After(30 * time.Second):
		fmt.Printf("timed out %v\n", cometId)

		comets.Lock()
		delete(comets.m, cometId)
		comets.Unlock()

		ret = CometResponse{"start-long-pool", cometId, Message{}}
	}
	response.WriteEntity(ret)
	fmt.Printf("Sending: %v\n", ret)
}
