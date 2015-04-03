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

package quickbase_test

import (
	quickbase "."
	"fmt"
	"os"
	"testing"
)

var _ = fmt.Println

func TestAuthentication(t *testing.T) {
	if _, err := authenticate(); err != nil {
		t.Error(err.Error())
		return
	}
}

func authenticate() (ticket quickbase.Ticket, err error) {
	return quickbase.Authenticate(os.Getenv("QUICKBASE_URL"),
		os.Getenv("QUICKBASE_USERNAME"),
		os.Getenv("QUICKBASE_PASSWORD"))
}

func TestDoQueryCount(t *testing.T) {
	var ticket quickbase.Ticket
	var err error
	if ticket, err = authenticate(); err != nil {
		t.Error(err)
	}
	if _, err := quickbase.DoQueryCount(ticket, os.Getenv("QUICKBASE_TABLE_DBID"), ""); err != nil {
		t.Error(err)
	}
}

func TestGetAppDTMInfo(t *testing.T) {
	received, nextAllowed, schemaModification, tableModifications, err := quickbase.GetAppDTMInfo(os.Getenv("QUICKBASE_URL"), os.Getenv("QUICKBASE_APP_DBID"))
	if err != nil {
		t.Error(err)
	}
	if received.After(nextAllowed) {
		t.Error("received %s is after nextAllowed %s", received, nextAllowed)
	}
	if schemaModification.SchemaModified.After(received) {
		t.Error("schemaModification.SchemaModified %s is after received %s", schemaModification.SchemaModified, received)
	}
	if schemaModification.RecordModified.After(received) {
		t.Error("schemaModification.RecordModified %s is after received %s", schemaModification.RecordModified, received)
	}
	for _, tableModification := range tableModifications {
		if tableModification.SchemaModified.After(received) {
			t.Error("tableModification.SchemaModified %s is after received %s", tableModification.SchemaModified, received)
		}
		if tableModification.RecordModified.After(received) {
			t.Error("tableModification.RecordModified %s is after received %s", tableModification.RecordModified, received)
		}
	}
	_, _, _, _, err = quickbase.GetAppDTMInfo(os.Getenv("QUICKBASE_URL"), "no-such-app-dbid")
	switch err := err.(type) {
	case nil:
		t.Error(fmt.Errorf("'no-such-app-dbid' should error out"))
	case quickbase.QuickBaseError:
		// 50 is the QuickBase error we expect for this fault; it indicates missing required value
		if err.Code != 50 {
			t.Error(err)
		}
	default:
		t.Error(err)
	}
}

func TestUserRoles(t *testing.T) {
	var ticket quickbase.Ticket
	var err error
	if ticket, err = authenticate(); err != nil {
		t.Error(err)
	}
	_, err = quickbase.UserRoles(ticket, os.Getenv("QUICKBASE_APP_DBID"))
	if err != nil {
		t.Error(err)
	}
}
