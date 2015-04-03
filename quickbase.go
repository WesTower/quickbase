// go-quickbase - Go bindings for Intuit's QuickBase
// Copyright (C) 2012-2014 WesTower Communications
// Copyright (C) 2014-2015 MasTec
//
// This file is part of go-quickbase.
//
// go-quickbase is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
// Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public
// License along with this program.  If not, see
// <http://www.gnu.org/licenses/>.

// Package go-quickbase provides access to Intuit's QuickBase API.
//
// QuickBase is a Web-accessible non-SQL relational database. While it
// does have some limitations and quirks, it is an extremely effective
// tool for rapid prototyping and development of end-user business
// applications.
package quickbase

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	xmlx "github.com/jteeuwen/go-pkg-xmlx"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QuickBaseError represents an error returned by the QuickBase API,
// as documented at
// <http://www.quickbase.com/api-guide/index.html#errorcodes.html>.
type QuickBaseError struct {
	Message string // human-readable message; corresponds to errtext in a response
	Code    int    // corresponds to errcode in a response
}

func (e QuickBaseError) Error() string {
	return e.Message
}

// A Ticket represents a QuickBase authentication ticket.
type Ticket struct {
	ticket   string
	userid   string
	url      string
	Apptoken string // if set, then each call using this Ticket
	// will include this Apptoken
}

// Authenticate authenticates a user to QuickBase; it's required
// before executing any other API call.  The username and password
// arguments are as documented at
// <http://www.quickbase.com/api-guide/index.html#authenticate.html>.
//
// Warning: URL must be of the form 'https://instance.quickbase.com/',
// to include the trailing slash.  It'd be nice to fix this someday to
// use a decent URL library to Do the Right Thing.
func Authenticate(url, username, password string) (ticket Ticket, err error) {
	doc, err := executeApiCall(url+"db/main", "API_Authenticate", map[string]string{"username": username, "password": password})
	if err != nil {
		return ticket, err
	}
	return Ticket{doc.SelectNode("", "ticket").GetValue(), doc.SelectNode("", "userid").GetValue(), url, ""}, nil
}

type apiParam struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type quickBaseRequest struct {
	XMLName xml.Name `xml:"qdbapi"`
	Params  []apiParam
}

func executeApiCall(url, api_call string, parameters map[string]string) (doc *xmlx.Document, err error) {
	count := 0
	for _, _ = range parameters {
		count++
	}
	api_params := make([]apiParam, count)
	i := 0
	for key, _ := range parameters {
		api_params[i] = apiParam{xml.Name{"", key}, parameters[key]}
		i++
	}
	req := quickBaseRequest{Params: api_params}
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

func executeRawApiCall(url, api_call string, parameters map[string]string) (resp *http.Response, err error) {
	count := 0
	for _, _ = range parameters {
		count++
	}
	api_params := make([]apiParam, count)
	i := 0
	for key, _ := range parameters {
		api_params[i] = apiParam{xml.Name{"", key}, parameters[key]}
		i++
	}
	req := quickBaseRequest{Params: api_params}
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
	return client.Do(http_req)
}

// SchemaModification represents the modification informatiom from
// GetAppDTMInfo
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

type nodeSelector interface {
	SelectNode(space, local string) *xmlx.Node
}

func selectNodeToTime(root nodeSelector, name string) (t time.Time, err error) {
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

// EditRecord edits a QuickBase record.  The fields argument is a map
// from field labels to the desired values.
func EditRecord(ticket Ticket, dbid string, recordId int, fields map[string]string) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	params["rid"] = fmt.Sprintf("%d", recordId)
	for field, value := range fields {
		params["_fnm_"+field] = value
	}
	_, err = executeApiCall(ticket.url+"db/"+dbid, "API_EditRecord", params)
	return err
}

// DoQueryCount returns the number of rows which would have been
// returned by DoQuery for the same query, or an error.
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

// DoStructuredQuery queries QuickBase, returning a map from field IDs
// to the field values for each result.  It has the advantage of being
// slightly more space-efficient for large queries than DoQuery, and
// not being prone to the field name/label confusion which hampers
// DoQuery.  All arguments are as in DoQuery.
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

// DoQuery queries QuickBase, returning a map from field labels to
// field values for each result.  The arguments dbid, query, clist,
// slist & options are all as documented at
// <http://www.quickbase.com/api-guide/index.html#do_query.html>.
//
// Warning: DoQuery can 'lose' fields if two fields have different
// names but the same label, e.g. 'foo ' and 'foo*' will have the same
// label 'foo_'.
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

// Warning: experimental
//
// DoQueryChan is intended to return a channel which will yield one
// result at a time, enabling streamed handling of large result sets.
// It has not been heavily tested, and may or may not currently work.
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
	api_params := make([]apiParam, len(params))
	i := 0
	for key, val := range params {
		api_params[i] = apiParam{xml.Name{"", key}, val}
		i++
	}
	req := quickBaseRequest{Params: api_params}
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

// GenResultTable queries QuickBase, returning the results an
// http.Response streaming the CSV response.  This can be one of the
// most efficient ways to retrieve a massive amount of data from
// QuickBase, with none of the overhead of the XML response format.
func GenResultsTable(ticket Ticket, dbid, query string, columns []int) (resp *http.Response, err error) {
	strCols := make([]string, len(columns))
	for i, col := range columns {
		strCols[i] = strconv.Itoa(col)
	}
	clist := strings.Join(strCols, ".")
	params := map[string]string{
		"clist":    clist,
		"options":  "csv",
		"slist":    "3",
		"ticket":   ticket.ticket,
		"apptoken": ticket.Apptoken,
	}
	if query != "" {
		params["query"] = query
	}
	return executeRawApiCall(ticket.url+"/db/"+dbid, "API_GenResultsTable", params)
}

// AddRecord adds a record; it uses the same conventions as
// EditRecord.  It returns the record ID of the newly-created record.
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
	ridNode := doc.SelectNode("", "rid")
	if ridNode == nil {
		return 0, fmt.Errorf("No rid returned from API_AddRecord")
	}
	return strconv.Atoi(ridNode.GetValue())
}

// DeleteRecord does what it says on the tin: deletes a particular
// record from a QuickBase table.
func DeleteRecord(ticket Ticket, dbid string, rid int) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	params["rid"] = strconv.Itoa(rid)
	_, err = executeApiCall(ticket.url+"db/"+dbid, "API_DeleteRecord", params)
	return err
}

// ChangeRecordOwner changes a record's owner, with arguments as
// documented at
// <http://www.quickbase.com/api-guide/index.html#change_record_owner.html>.
func ChangeRecordOwner(ticket Ticket, dbid string, rid int, owner string) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	params["rid"] = strconv.Itoa(rid)
	params["newowner"] = owner
	_, err = executeApiCall(ticket.url+"db/"+dbid, "API_ChangeRecordOwner", params)
	return err
}

type User struct {
	Id   string
	Name string
	//Roles []Role
}

/*
not needed yet
type Role struct {
	Id       int
	Name     string
	Accesses []Access
}

type Access struct {
	Id   int
	Name string
}*/

// UserRoles will eventually return users with their roles; right now
// it just returns the user's IDs and name.
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

// Download retrieves a file from QuickBase, per
// <http://www.quickbase.com/api-guide/index.html>.
func Download(ticket Ticket, dbid string, rid, fid, vid int) (file io.ReadCloser, err error) {
	url := fmt.Sprintf("%sup/%s/a/r%d/e%d/v%d?ticket=%s&apptoken=%s", ticket.url, dbid, rid, fid, vid, ticket.ticket, ticket.Apptoken)
	if response, err := http.Get(url); err != nil {
		return nil, err
	} else {
		return response.Body, nil
	}
}

// Upload uploads a single file to a field in a QuickBase record.
func Upload(ticket Ticket, dbid string, rid, fid int, filename string, r io.Reader) (err error) {
	// Since Go is strongly-typed and I've not defined an
	// interface for field values yet, files must be individually uploaded
	// to records.  This is a prime opportunity for refactoring.
	reqReader, reqWriter := io.Pipe()
	client := &http.Client{}
	http_req, err := http.NewRequest("POST", ticket.url+"db/"+dbid, reqReader)
	if err != nil {
		return err
	}
	http_req.Header.Add("QUICKBASE-ACTION", "API_EditRecord")
	http_req.Header.Add("Content-Type", "application/xml")
	go func() {
		fmt.Fprintf(reqWriter, "<qdbapi><ticket>%s</ticket><apptoken>%s</apptoken><rid>%d</rid><field fid='%d' filename='%s'>",
			ticket.ticket, ticket.Apptoken, rid, fid, filename)
		encoder := base64.NewEncoder(base64.StdEncoding, reqWriter)
		io.Copy(encoder, r)
		encoder.Close() // flush & close the encoder, so that all data are sent
		fmt.Fprintf(reqWriter, "</field></qdbapi>")
		reqWriter.Close()
	}()
	resp, err := client.Do(http_req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// FIXME: do we need to go through this rigamarole, or can we just return above?
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

// ImportFromCSV imports a CSV into QuickBase.  It expects the CSV not
// to have a header line.  The columns argument becomes the clist
// documented in
// <http://www.quickbase.com/api-guide/index.html#importfromcsv.html>
func ImportFromCSV(ticket Ticket, dbid string, columns []int, r io.Reader) (err error) {
	params := map[string]string{"ticket": ticket.ticket}
	if ticket.Apptoken != "" {
		params["apptoken"] = ticket.Apptoken
	}
	strCols := make([]string, len(columns))
	for i, col := range columns {
		strCols[i] = strconv.Itoa(col)
	}
	params["clist"] = strings.Join(strCols, ".")
	params["skipfirst"] = "1"
	// FIXME: it'd be nice to stream this, but how to properly escape CDATA in the CSV?
	var csv []byte
	if csv, err = ioutil.ReadAll(r); err != nil {
		return
	}
	params["records_csv"] = string(csv)
	_, err = executeApiCall(ticket.url+"db/"+dbid, "API_ImportFromCSV", params)
	return err
}
