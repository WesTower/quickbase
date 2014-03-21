package quickbase

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	xmlx "github.com/jteeuwen/go-pkg-xmlx"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"os"
)

type AuthRequest struct {
	XMLName  xml.Name `xml:"qdbapi"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Hours    int      `xml:"hours"`
}

type AuthReply struct {
	XMLName xml.Name `xml:"qdbapi"`
}

type QuickBaseError struct {
	Message string
	Code    int
}

func (e QuickBaseError) Error() string {
	return e.Message
}

type Ticket struct {
	ticket   string
	userid   string
	url      string
	Apptoken string
}

type Field struct {
	Name string
	Id   int
}

func Authenticate(url, username, password string) (ticket Ticket, err error) {
	doc, err := executeApiCall(url+"db/main", "API_Authenticate", map[string]string{"username": username, "password": password})
	if err != nil {
		return ticket, err
	}
	return Ticket{doc.SelectNode("", "ticket").GetValue(), doc.SelectNode("", "userid").GetValue(), url, ""}, nil
}

type ApiParam struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type QuickBaseRequest struct {
	XMLName xml.Name `xml:"qdbapi"`
	Params  []ApiParam
}

func executeApiCall(url, api_call string, parameters map[string]string) (doc *xmlx.Document, err error) {
	count := 0
	for _, _ = range parameters {
		count++
	}
	api_params := make([]ApiParam, count)
	i := 0
	for key, _ := range parameters {
		api_params[i] = ApiParam{xml.Name{"", key}, parameters[key]}
		i++
	}
	req := QuickBaseRequest{Params: api_params}
	xml_req, err := xml.Marshal(req)
	if err != nil {
		return
	}
	client := &http.Client{}
	http_req, err := http.NewRequest("POST", url, bytes.NewReader(xml_req))
	if err != nil {
		return nil, err
	}
	http_req.Header.Add("QUICKBASE-ACTION", api_call)
	http_req.Header.Add("Content-Type", "application/xml")
	resp, err := client.Do(http_req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	//tee := io.TeeReader(resp.Body, os.Stderr)
	doc = xmlx.New()
	err = doc.LoadStream(resp.Body, nil)
	//err = doc.LoadStream(tee, nil)
	if err != nil {
		return nil, err
	}
	if errcode := doc.SelectNode("", "errcode").GetValue(); errcode != "0" {
		//err = fmt.Errorf(doc.SelectNode("", "errtext").GetValue())
		code, err := strconv.Atoi(errcode)
		if err != nil {
			return nil, err
		}
		return nil, QuickBaseError{Message: doc.SelectNode("", "errtext").GetValue(), Code: code}
	}

	return doc, nil
}

type SchemaModification struct {
	Dbid           string
	SchemaModified time.Time
	RecordModified time.Time
}

// GetAppDTMInfo returns the time the server received the request, the
// time the server will allow another request, the app schema
// modification date and table modification dates
func GetAppDTMInfo(baseUrl, dbid string) (received, nextAllowed time.Time, schemaModification SchemaModification, tableModification []SchemaModification, err error) {
	params := map[string]string{"dbid": dbid}
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		return
	}
	parsedUrl.Path = "/db/main"
	reqUrl := parsedUrl.String()
	doc, err := executeApiCall(reqUrl, "API_GetAppDTMInfo", params)
	if err != nil {
		return
	}
	errCode := doc.SelectNode("", "errcode")
	if errCode.GetValue() != "0" {
		errText := doc.SelectNode("", "errtext")
		err = fmt.Errorf("Error %s: %s", errCode, errText)
		return
	}
	received, err = selectNodeToTime(doc, "RequestTime")
	if err != nil {
		return
	}
	nextAllowed, err = selectNodeToTime(doc, "RequestNextAllowedTime")
	if err != nil {
		return
	}
	app := doc.SelectNode("", "app")
	if app == nil {
		err = fmt.Errorf("No app returned")
		return
	}
	dbid = ""
	for _, attr := range app.Attributes {
		if attr.Name.Space == "" && attr.Name.Local == "id" {
			dbid = attr.Value
			break
		}
	}
	if dbid == "" {
		err = fmt.Errorf("Missing table dbid in app")
		return received, nextAllowed, schemaModification, tableModification, err
	}
	schemaModification.Dbid = dbid
	schemaModification.SchemaModified, err = selectNodeToTime(app, "lastModifiedTime")
	if err != nil {
		return
	}
	schemaModification.RecordModified, err = selectNodeToTime(app, "lastRecModTime")
	if err != nil {
		return
	}
	tablesNode := doc.SelectNode("", "tables")
	if tablesNode == nil {
		err = fmt.Errorf("No tables returned")
		return
	}
	tables := tablesNode.SelectNodes("", "table")
	for _, table := range tables {
		var dbid string
		for _, attr := range table.Attributes {
			if attr.Name.Space == "" && attr.Name.Local == "id" {
				dbid = attr.Value
				break
			}
		}
		if dbid == "" {
			err = fmt.Errorf("Missing table dbid in table")
			return received, nextAllowed, schemaModification, tableModification, err
		}
		schemaMod, err := selectNodeToTime(table, "lastModifiedTime")
		if err != nil {
			return received, nextAllowed, schemaModification, tableModification, err
		}
		lastRecMod, err := selectNodeToTime(table, "lastRecModTime")
		if err != nil {
			return received, nextAllowed, schemaModification, tableModification, err
		}
		tableModification = append(tableModification, SchemaModification{dbid, schemaMod, lastRecMod})
	}
	return
}

type NodeSelector interface {
	SelectNode(space, local string) *xmlx.Node
}

func selectNodeToTime(root NodeSelector, name string) (t time.Time, err error) {
	if root == nil {
		return t, fmt.Errorf("Nil root passed in")
	}
	node := root.SelectNode("", name)
	if node == nil {
		return t, fmt.Errorf("Tag named %s not found", name)
	}
	if msecs, err := strconv.ParseInt(node.GetValue(), 10, 64); err != nil {
		return t, err
	} else {
		return time.Unix(msecs/1000, (msecs%1000)*1000), nil
	}
	panic("can't get here, silly Go 1.0")
}

func EditRecord(ticket Ticket, dbid string, recordId int, fields map[string]string) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	params["rid"] = fmt.Sprintf("%d", recordId)
	for field, value := range fields {
		params["_fnm_"+field] = value
	}
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_EditRecord", params)
	if err != nil {
		return err
	}
	errCode := doc.SelectNode("", "errcode")
	if errCode.GetValue() != "0" {
		errText := doc.SelectNode("", "errtext")
		return fmt.Errorf("Error %s: %s", errCode, errText)
	}
	return nil
}

func DoQueryCount(ticket Ticket, dbid, query string) (count int64, err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	if query != "" {
		params["query"] = query
	}
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_DoQueryCount", params)
	if err != nil {
		return count, err
	}
	countNode := doc.SelectNode("", "numMatches")
	if countNode == nil {
		return 0, fmt.Errorf("Invalid replay from QuickBase")
	}
	return strconv.ParseInt(countNode.GetValue(), 10, 64)
}

func DoStructuredQuery(ticket Ticket, dbid, query, clist, slist, options string) (records []map[int]string, err error) {
	params := map[string]string{"ticket": ticket.ticket, "fmt": "structured"}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	if query != "" {
		params["query"] = query
	}
	if clist != "" {
		params["clist"] = clist
	}
	if slist != "" {
		params["slist"] = slist
	}
	if options != "" {
		params["options"] = options
	}
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_DoQuery", params)
	if err != nil {
		return nil, err
	}
	for _, record := range doc.SelectNodes("", "record") {
		record_map := make(map[int]string)
		for _, child := range record.Children {

			record_map[child.Ai("", "id")] = child.GetValue()
		}
		records = append(records, record_map)
	}
	return
}

func DoQuery(ticket Ticket, dbid, query, clist, slist, options string) (records []map[string]string, err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	if query != "" {
		params["query"] = query
	}
	if clist != "" {
		params["clist"] = clist
	}
	if slist != "" {
		params["slist"] = slist
	}
	if options != "" {
		params["options"] = options
	}
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_DoQuery", params)
	if err != nil {
		return nil, err
	}
	for _, record := range doc.SelectNodes("", "record") {
		record_map := make(map[string]string)
		for _, child := range record.Children {
			// Each child is a particular field.  A
			// multi-line field may have multiple text
			// nodes, separated by "<br/>" nodes.  This
			// means that we need to collect up the values
			// of all text children, and interpolate
			// newlines where necessary.
			//record_map[child.Name.Local] = child.GetValue()
			for _, grandchild := range child.Children {
				switch grandchild.Type {
				case xmlx.NT_TEXT:
					record_map[child.Name.Local] += grandchild.Value
				case xmlx.NT_ELEMENT:
					if grandchild.Name.Local == "BR" {
						// apparently, QuickBase internally uses carriage returns to separate lines
						record_map[child.Name.Local] += "\r"
					} else {
						return nil, fmt.Errorf("Cannot handle tag %s within value for field %s", grandchild.Name.Local, child.Name.Local)
					}
				default:
					return nil, fmt.Errorf("Cannot handle non-text, non-element within value for field %s", child.Name.Local)
				}
			}
		}
		records = append(records, record_map)
	}
	return
}

func DoQueryChan(ticket Ticket, dbid, query, clist, slist string) (records chan map[string]string, err error) {
	records = make(chan map[string]string)
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	if query != "" {
		params["query"] = query
	}
	if clist != "" {
		params["clist"] = clist
	}
	if slist != "" {
		params["slist"] = slist
	}
	api_params := make([]ApiParam, len(params))
	i := 0
	for key, val := range params {
		api_params[i] = ApiParam{xml.Name{"", key}, val}
		i++
	}
	req := QuickBaseRequest{Params: api_params}
	pipe_reader, pipe_writer := io.Pipe()
	http_req, err := http.NewRequest("POST", ticket.url+"db/"+dbid, pipe_reader)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	http_req.Header.Add("QUICKBASE-ACTION", "API_DoQuery")
	http_req.Header.Add("Content-Type", "application/xml")
	go func() {
		encoder := xml.NewEncoder(pipe_writer)
		encoder.Encode(req)
		pipe_writer.Close()
	}()
	resp, err := client.Do(http_req)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(resp.Body)
	for token, err := decoder.Token(); err != io.EOF; token, err = decoder.Token() {
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.ProcInst:
			// skip
		case xml.StartElement:
			if token.Name.Local != "qdbapi" {
				return nil, fmt.Errorf("qdbapi expected; %s found", token.Name.Local)
			}
			qb_errcode := false
			qb_errtext := ""
			last_record_len := 1
			for token, err := decoder.Token(); err != io.EOF; token, err = decoder.Token() {
				switch token := token.(type) {
				case xml.StartElement:
					switch token.Name.Local {
					case "errcode":
						token, err := decoder.Token()
						if err != nil {
							return nil, err
						}
						if string(token.(xml.CharData)) != "0" {
							qb_errcode = true
							if qb_errtext != "" {
								return nil, fmt.Errorf(qb_errtext)
							}
						}
					case "errtext":
						token, err := decoder.Token()
						if err != nil {
							return nil, err
						}
						qb_errtext = string(token.(xml.CharData))
						if qb_errcode {
							return nil, fmt.Errorf(qb_errtext)
						}
					case "record":
						go func() {
							defer resp.Body.Close()

							record := make(map[string]string, last_record_len)
							last_field := ""
							last_data := ""
							in_record := true
						record:
							for token, err := decoder.Token(); err != io.EOF; token, err = decoder.Token() {
								switch token := token.(type) {
								case xml.StartElement:
									switch {
									case in_record == true:
										last_data = ""
										last_field = token.Name.Local
									case token.Name.Local != "record":
										close(records)
										break record
									default:
										in_record = true
									}
								case xml.EndElement:
									switch {
									case !in_record && token.Name.Local == "qdbapi":
										close(records)
										break record
									case in_record && token.Name.Local == "record":
										in_record = false
										records <- record
									default:
										record[last_field] = last_data
										last_field = ""
									}
								case xml.CharData:
									last_data += string(token)
								}
							}

						}()
						return records, nil
					}
				}
			}
		}
	}
	panic("should never have gotten here")
}

type AuthInfo struct {
	Ticket   Ticket
	Apptoken string
}

/*
func DumpTable(authinfo AuthInfo, table string, columns []int, path) {
	// try to pull in the entire table if possible
	result := executeRawApiCall(authinfo.Ticket.url + '/db/' + table, \
		'API_GenResultsTable',
		map[string][string]{
			"clist":clist,
			"options":"csv",
			"slist":"3",
			"ticket":authinfo.Ticket.ticket,
			"apptoken":authinfo.Apptoken
		})
	}*/

// AddRecord adds a record; it uses the same conventions as EditRecord.
func AddRecord(ticket Ticket, dbid string, fields map[string]string) (rid int, err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	for field, value := range fields {
		params["_fnm_"+field] = value
	}
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_AddRecord", params)
	if err != nil {
		return 0, err
	}
	errCode := doc.SelectNode("", "errcode")
	if errCode.GetValue() != "0" {
		errText := doc.SelectNode("", "errtext")
		return 0, fmt.Errorf("Error %s: %s", errCode, errText)
	}
	ridNode := doc.SelectNode("", "rid")
	if ridNode == nil {
		return 0, fmt.Errorf("No rid returned from API_AddRecord")
	}
	return strconv.Atoi(ridNode.GetValue())
}

func DeleteRecord(ticket Ticket, dbid string, rid int) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	params["rid"] = strconv.Itoa(rid)
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_DeleteRecord", params)
	if err != nil {
		return err
	}
	errCode := doc.SelectNode("", "errcode")
	if errCode.GetValue() != "0" {
		errText := doc.SelectNode("", "errtext")
		return fmt.Errorf("Error %s: %s", errCode, errText)
	}
	return nil
}

func ChangeRecordOwner(ticket Ticket, dbid string, rid int, owner string) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	params["rid"] = strconv.Itoa(rid)
	params["newowner"] = owner
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_ChangeRecordOwner", params)
	if err != nil {
		return err
	}
	errCode := doc.SelectNode("", "errcode")
	if errCode.GetValue() != "0" {
		errText := doc.SelectNode("", "errtext")
		return fmt.Errorf("Error %s: %s", errCode, errText)
	}
	return nil
}

type User struct {
	Id string
	Name string
	//Roles []Role
}

type Role struct {
	Id int
	Name string
	Accesses []Access
}

type Access struct {
	Id int
	Name string
}

func UserRoles(ticket Ticket, dbid string) (users []User, err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	doc, err := executeApiCall(ticket.url+"db/"+dbid, "API_UserRoles", params)
	if err != nil {
		return nil, err
	}
	for _, userNode := range doc.SelectNodes("", "user") {
		user := User{Id: userNode.As("", "id"), Name: userNode.S("", "name")}
		users = append(users, user)
	}
	return users, nil
}

func Download(ticket Ticket, dbid string, rid, fid, vid int) (file io.ReadCloser, err error) {
	url := fmt.Sprintf("%sup/%s/a/r%d/e%d/v%d?ticket=%s&apptoken=%s", ticket.url, dbid, rid, fid, vid, ticket.ticket, ticket.Apptoken)
	if response, err := http.Get(url); err != nil {
		return nil, err
	} else {
		return response.Body, nil
	}
}

// Since Go is strongly-typed and I've not defined an interface for
// field values yet, files must be individually uploaded to records.
// This is a prime opportunity for refactoring.
func Upload(ticket Ticket, dbid string, rid, fid int, filename string, r io.Reader) (err error) {
	fmt.Println("uploading", ticket.url+"db/"+dbid)
	reqReader, reqWriter := io.Pipe()
	client := &http.Client{}
	http_req, err := http.NewRequest("POST", ticket.url+"db/"+dbid, reqReader)
	if err != nil {
		return err
	}
	http_req.Header.Add("QUICKBASE-ACTION", "API_EditRecord")
	http_req.Header.Add("Content-Type", "application/xml")
	rw := io.MultiWriter(reqWriter, os.Stderr)
	go func() {
		fmt.Fprintf(rw, "<qdbapi><ticket>%s</ticket><apptoken>%s</apptoken><rid>%d</rid><field fid='%d' filename='%s'>", 
			ticket.ticket, ticket.Apptoken, rid, fid, filename)
		fmt.Println("encoding")
		encoder := base64.NewEncoder(base64.StdEncoding, rw)
		io.Copy(encoder, r)
		fmt.Fprintf(rw, "</field></qdbapi>")
		fmt.Println("encoded")
		reqWriter.Close()
	}()
	fmt.Println("About to upload")
	resp, err := client.Do(http_req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("uploaded")

	//tee := io.TeeReader(resp.Body, os.Stderr)
	doc := xmlx.New()
	err = doc.LoadStream(resp.Body, nil)
	//err = doc.LoadStream(tee, nil)
	if err != nil {
		return err
	}
	if errcode := doc.SelectNode("", "errcode").GetValue(); errcode != "0" {
		err = fmt.Errorf(doc.SelectNode("", "errtext").GetValue())
		return
	}
	return nil
}
