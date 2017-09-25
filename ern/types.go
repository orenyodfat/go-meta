// This file is part of the go-meta library.
//
// Copyright (C) 2017 JAAK MUSIC LTD
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// If you have any questions please contact yo@jaak.io

package ern

type Context map[string]string

var PartyDetailsConext = Context{
	"partyId": "http://service.ddex.net/xml/ern/382/release-notification.xsd#PartyId",	
	"isIsni": "http://service.ddex.net/xml/ern/382/release-notification.xsd#isIsni",// xs:boolean
	"isDPid": "http://service.ddex.net/xml/ern/382/release-notification.xsd#isDPid",// xs:boolean
	"namespace": "http://service.ddex.net/xml/ern/382/release-notification.xsd#namespace",
	"fullName": "http://service.ddex.net/xml/ern/382/release-notification.xsd#fullName", //ern:Name
	"abbreviatedName": "http://service.ddex.net/xml/ern/382/release-notification.xsd#abbreviatedName", //ern:Name
	"role": "http://service.ddex.net/xml/ern/382/release-notification.xsd#role",
	"userDefinedValue": "http://service.ddex.net/xml/ern/382/release-notification.xsd#userDefinedValue", //xs:string
}

// PartyDetails combines the PartyName and PartyId DDEX complex types.
type PartyDetails struct {
	Context	 					Context `json:"@context"`
	PartyId  					int64 `json:"partyId, omitempty"`
	IsIsni   					bool `json:"isIsni, omitempty"`
	IsDPid   					bool `json:"isDPid, omitempty"`
	Namespace   			string `json:"namespace, omitempty"`
	FullName   				string `json:"fullName, omitempty"`
	AbbreviatedName   string `json:"abbreviatedName, omitempty"`
	Role   						string `json:"role, omitempty"`
	UserDefinedValue  string `json:"userDefinedValue, omitempty"`
}