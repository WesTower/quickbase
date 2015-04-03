# quickbase
--
    import "."

Package go-quickbase provides access to Intuit's QuickBase API.

QuickBase is a Web-accessible non-SQL relational database. While it does have
some limitations and quirks, it is an extremely effective tool for rapid
prototyping and development of end-user business applications.

## Usage

#### func  AddRecord

```go
func AddRecord(ticket Ticket, dbid string, fields map[string]string) (rid int, err error)
```
AddRecord adds a record; it uses the same conventions as EditRecord. It returns
the record ID of the newly-created record.

#### func  ChangeRecordOwner

```go
func ChangeRecordOwner(ticket Ticket, dbid string, rid int, owner string) (err error)
```
ChangeRecordOwner changes a record's owner, with arguments as documented at
<http://www.quickbase.com/api-guide/index.html#change_record_owner.html>.

#### func  DeleteRecord

```go
func DeleteRecord(ticket Ticket, dbid string, rid int) (err error)
```
DeleteRecord does what it says on the tin: deletes a particular record from a
QuickBase table.

#### func  DoQuery

```go
func DoQuery(ticket Ticket, dbid, query, clist, slist, options string) (records []map[string]string, err error)
```
DoQuery queries QuickBase, returning a map from field labels to field values for
each result. The arguments dbid, query, clist, slist & options are all as
documented at <http://www.quickbase.com/api-guide/index.html#do_query.html>.

Warning: DoQuery can 'lose' fields if two fields have different names but the
same label, e.g. 'foo ' and 'foo*' will have the same label 'foo_'.

#### func  DoQueryChan

```go
func DoQueryChan(ticket Ticket, dbid, query, clist, slist string) (records chan map[string]string, err error)
```
Warning: experimental

DoQueryChan is intended to return a channel which will yield one result at a
time, enabling streamed handling of large result sets. It has not been heavily
tested, and may or may not currently work.

#### func  DoQueryCount

```go
func DoQueryCount(ticket Ticket, dbid, query string) (count int64, err error)
```
DoQueryCount returns the number of rows which would have been returned by
DoQuery for the same query, or an error.

#### func  DoStructuredQuery

```go
func DoStructuredQuery(ticket Ticket, dbid, query, clist, slist, options string) (records []map[int]string, err error)
```
DoStructuredQuery queries QuickBase, returning a map from field IDs to the field
values for each result. It has the advantage of being slightly more
space-efficient for large queries than DoQuery, and not being prone to the field
name/label confusion which hampers DoQuery. All arguments are as in DoQuery.

#### func  Download

```go
func Download(ticket Ticket, dbid string, rid, fid, vid int) (file io.ReadCloser, err error)
```
Download retrieves a file from QuickBase, per
<http://www.quickbase.com/api-guide/index.html>.

#### func  EditRecord

```go
func EditRecord(ticket Ticket, dbid string, recordId int, fields map[string]string) (err error)
```
EditRecord edits a QuickBase record. The fields argument is a map from field
labels to the desired values.

#### func  GenResultsTable

```go
func GenResultsTable(ticket Ticket, dbid, query string, columns []int) (resp *http.Response, err error)
```
GenResultTable queries QuickBase, returning the results an http.Response
streaming the CSV response. This can be one of the most efficient ways to
retrieve a massive amount of data from QuickBase, with none of the overhead of
the XML response format.

#### func  GetAppDTMInfo

```go
func GetAppDTMInfo(baseUrl, dbid string) (received, nextAllowed time.Time, schemaModification SchemaModification, tableModification []SchemaModification, err error)
```
GetAppDTMInfo returns the time the server received the request, the time the
server will allow another request, the app schema modification date and table
modification dates

#### func  ImportFromCSV

```go
func ImportFromCSV(ticket Ticket, dbid string, columns []int, r io.Reader) (err error)
```
ImportFromCSV imports a CSV into QuickBase. It expects the CSV not to have a
header line. The columns argument becomes the clist documented in
<http://www.quickbase.com/api-guide/index.html#importfromcsv.html>

#### func  Upload

```go
func Upload(ticket Ticket, dbid string, rid, fid int, filename string, r io.Reader) (err error)
```
Upload uploads a single file to a field in a QuickBase record.

#### func  UserRoles

```go
func UserRoles(ticket Ticket, dbid string) (users []User, err error)
```
UserRoles will eventually return users with their roles; right now it just
returns the user's IDs and name.

#### type QuickBaseError

```go
type QuickBaseError struct {
	Message string // human-readable message; corresponds to errtext in a response
	Code    int    // corresponds to errcode in a response
}
```

QuickBaseError represents an error returned by the QuickBase API, as documented
at <http://www.quickbase.com/api-guide/index.html#errorcodes.html>.

#### func (QuickBaseError) Error

```go
func (e QuickBaseError) Error() string
```

#### type SchemaModification

```go
type SchemaModification struct {
	Dbid           string
	SchemaModified time.Time
	RecordModified time.Time
}
```

SchemaModification represents the modification informatiom from GetAppDTMInfo

#### type Ticket

```go
type Ticket struct {
	Apptoken string // if set, then each call using this Ticket
}
```

A Ticket represents a QuickBase authentication ticket.

#### func  Authenticate

```go
func Authenticate(url, username, password string) (ticket Ticket, err error)
```
Authenticate authenticates a user to QuickBase; it's required before executing
any other API call. The username and password arguments are as documented at
<http://www.quickbase.com/api-guide/index.html#authenticate.html>.

Warning: URL must be of the form 'https://instance.quickbase.com/', to include
the trailing slash. It'd be nice to fix this someday to use a decent URL library
to Do the Right Thing.

#### type User

```go
type User struct {
	Id   string
	Name string
}
```
