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

package eidr

import (
	//"context"
	//"encoding/csv"
	"io"
	//"strconv"

	"github.com/ipfs/go-cid"
	"github.com/meta-network/go-meta"
	//"github.com/meta-network/go-meta/doi"
	metaxml "github.com/meta-network/go-meta/xml"
	"github.com/meta-network/go-meta/xmlschema"
)

// Converter converts DDEX ERN XML files into META objects.
type Converter struct {
	store *meta.Store
}

// NewConverter returns a Converter which stores META objects in the given META
// store.
func NewConverter(store *meta.Store) *Converter {
	return &Converter{
		store: store,
	}
}

//
//func (c *Converter) ConvertEIDRCSV(ctx context.Context, src io.Reader, outStream chan *cid.Cid) error {
//	var err error
//	r := csv.NewReader(src)
//	r.Comma = '\t'
//	_, err = r.Read()
//	if err != nil {
//		return err
//	}
//	for err == nil {
//		var fields []string
//		if fields, err = r.Read(); err == nil {
//			var offset int
//			baseobj := BaseObjectData{}
//			baseobj.Context = BaseObjectContext
//			baseobj.ID = doi.ID(fields[0])
//			baseobj.StructuralType = fields[1]
//			baseobj.ReferentType = fields[3]
//			baseobj.ResourceName = fields[4]
//			offset, err := strconv.Atoi(fields[129])
//			if err == nil {
//				for i := 130; i < 130+(offset*4); i += 4 {
//					altid := AlternateID{
//						ID:       fields[i],
//						Type:     fields[i+1],
//						Relation: fields[i+3],
//					}
//					if altid.Type == "Proprietary" {
//						altid.Domain = fields[i+2]
//					}
//					baseobj.AlternateID = append(baseobj.AlternateID, altid)
//				}
//			}
//			if baseobj.ReferentType == "Series" {
//				//baseobj.ExtraMetaData = interface{}(SeriesInfo{
//				seriesobj := SeriesInfo{
//					SeriesClass: fields[180],
//				}
//				obj, err := meta.Encode(seriesobj)
//				if err != nil {
//					return err
//				}
//				if err := c.store.Put(obj); err != nil {
//					return err
//				}
//				baseobj.ExtraMetaData = *obj.Cid()
//			} else if fields[184] != "0" { // "Seas Parent"
//				seasonobj := SeasonInfo{}
//				if fields[192] != "" { // "NumReq"
//					seasonobj.NumberRequired = true
//				}
//			}
//			obj, err := meta.Encode(baseobj)
//			if err != nil {
//				return err
//			}
//			if err := c.store.Put(obj); err != nil {
//				return err
//			}
//			// send the object's CID to the output stream
//			select {
//			case outStream <- obj.Cid():
//			case <-ctx.Done():
//				return ctx.Err()
//			}
//		}
//	}
//	return nil
//}

// TODO: when we get xml data
func (c *Converter) ConvertEIDRXML(src io.Reader) (*cid.Cid, error) {
	context := []*cid.Cid{
		xmlschema.EIDR_common.Cid,
		xmlschema.EIDR_md.Cid,
	}
	obj, err := metaxml.EncodeXML(src, context, c.store.Put)
	if err != nil {
		return nil, err
	}

	return obj.Cid(), nil
}
