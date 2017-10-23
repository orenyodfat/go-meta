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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	graphql "github.com/neelance/graphql-go"
)

// Resolver is a general purpose GraphQL resolver function,
// retrieves data from a META store and SQLite3 index
type Resolver struct {
	schemas map[string]*graphql.Schema
}

// NewResolver returns a new resolver for returning data from the given
// META store and SQLite3 index
func NewResolver(schemas map[string]*graphql.Schema) *Resolver {
	return &Resolver{schemas}
}

// GraphQLSchema is the GraphQL schema for META music domains.
const GraphQLSchema = `

# This GraphQL schema defines the META Media types and queries.
#
# It consists of the following main entities:
#
# MusicPerformer
# MusicComposer
# RecordLabel
# MusicPublisher
# MusicRecording
# MusicWork
# MusicRelease
# MusicProduct
#
# Each entity has a set of widely used identifiers which are used to uniquely
# identify them:
#
# ISNI - International Standard Name Identifier
# IPI  - Interested Parties Information
# DPID - DDEX Party Identifier
# ISRC - International Standard Recording Code
# ISWC - International Standard Musical Work Code
# GRid - Global Release Identifier
# UPC  - Universal Product Code
# EAN  - International Article Number
#
# Other entity properties (like performer name or recording title) are linked
# to these identifiers by various sources in the META network.

#
# --- Main Entities ---
#
type MusicPerformer {
  identifiers: [PartyIdentifier]

  name: StringValue

  products:   [MusicProductLink]
  releases:   [MusicReleaseLink]
  recordings: [MusicRecordingLink]
  labels:     [RecordLabelLink]
}

type MusicComposer {
  identifiers: [PartyIdentifier]

  name: StringValue

  works:      [MusicWorkLink]
  publishers: [MusicPublisherLink]
}

type RecordLabel {
  identifiers: [PartyIdentifier]

  name: StringValue

  products:   [MusicProductLink]
  releases:   [MusicReleaseLink]
  recordings: [MusicRecordingLink]
  performers: [MusicPerformerLink]
}

type MusicPublisher {
  identifiers: [PartyIdentifier]

  name: StringValue

  composers: [MusicComposerLink]
  works:     [MusicWorkLink]
}

type MusicRecording {
  isrc: IDValue

  title: StringValue

  products:   [MusicProductLink]
  releases:   [MusicReleaseLink]
  performers: [MusicPerformerLink]
  labels:     [RecordLabelLink]
  works:      [MusicWorkLink]
}

type MusicWork {
  iswc: IDValue

  title: StringValue

  composers:  [MusicComposerLink]
  recordings: [MusicRecordingLink]
  publishers: [MusicPublisherLink]
}

type MusicRelease {
  grid: IDValue

  title: StringValue

  products:   [MusicProductLink]
  recordings: [MusicRecordingLink]
  performers: [MusicPerformerLink]
  labels:     [RecordLabelLink]
	detailsByTerritory: [ReleaseDetailsByTerritoryLink]
}

type MusicProduct {
  upc: IDValue
  ean: IDValue

  releases:   [MusicReleaseLink]
  recordings: [MusicRecordingLink]
  performers: [MusicPerformerLink]
  labels:     [RecordLabelLink]
}

type ReleaseDetailsByTerritory {

}

#
# --- Link Types ---
#
type MusicPerformerLink {
  source:    Source!
  performer: MusicPerformer!
  role:      String
}

type MusicComposerLink {
  source:   Source!
  composer: MusicComposer!
}

type RecordLabelLink {
  source: Source!
  label:  RecordLabel!
}

type MusicPublisherLink {
  source:    Source!
  publisher: MusicPublisher!
}

type MusicRecordingLink {
  source:    Source!
  recording: MusicRecording!
}

type MusicWorkLink {
  source: Source!
  work:   MusicWork!
}

type MusicReleaseLink {
  source:  Source!
  release: MusicRelease!
}

type MusicProductLink {
  source:  Source!
  product: MusicProduct!
}

type ReleaseDetailsByTerritoryLink {
  source:  Source!
	displayTitle:String!
	displayArtist:String
	genre:String
}

#
# --- Value Types ---
#
enum PartyIdentifierType {
  ISNI
  IPI
  DPID
}

type PartyIdentifier {
  type:   PartyIdentifierType!
  value:  String!
  source: Source!
}

type StringValue {
  value:   String!
  sources: [StringSource]
}

type StringSource {
  value:  String!
  source: Source!
  score:  Int!
}

type Source {
  name: String!
}
type IDValue {
  value:   String!
  sources: [ValueSource]
}
type ValueSource {
  value:  String!
  source: Source!
  score:  Int!
}
`

// RecordingArgs are the arguments for a GraphQL musicRecording query.
type musicRecordingArgs struct {
	Title *string
	ID    *string
}

func formatJSON(data []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	formatted, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

// MusicRecording is a GraphQL resolver function which retrieves musicRecording object from a
// grahql schema which can be either ern,cwr or musicbrainz.
func (g *Resolver) MusicRecording(args musicRecordingArgs) (resolver *musicRecordingResolver, err error) {
	var queryString string
	switch {
	case args.Title != nil:
		queryString = fmt.Sprintf("{ soundRecording(title: %q){source title genre artistName soundRecordingId territoryCode }}", *args.Title)
	case args.ID != nil:
		queryString = fmt.Sprintf("{ soundRecording(id: %q){source title genre artistName soundRecordingId territoryCode }}", *args.ID)
	default:
		return nil, errors.New("missing title or id argument")
	}
	result := g.schemas["ern"].Exec(context.Background(), queryString, "", nil)
	if len(result.Errors) != 0 {
		return nil, result.Errors[0]
	}

	type Response struct {
		SoundRecording []map[string]string `json:"soundRecording"`
	}
	res := Response{}
	json.Unmarshal(result.Data, &res)

	musicRecording := MusicRecording{}
	for _, soundRecording := range res.SoundRecording {
		musicRecording.Genre.sources = append(musicRecording.Genre.sources, StringSource{value: soundRecording["genre"], source: soundRecording["source"], score: "5"})
		musicRecording.Title.sources = append(musicRecording.Title.sources, StringSource{value: soundRecording["title"], source: soundRecording["source"], score: "5"})
		musicRecording.Identifiers = append(musicRecording.Identifiers, Identifier{ID: soundRecording["soundRecordingId"], Type: "ISRC"})
	}
	musicRecording.Title.value, err = pickBestValue(musicRecording.Title.sources)
	if err != nil {
		return nil, err
	}
	musicRecording.Genre.value, err = pickBestValue(musicRecording.Genre.sources)
	if err != nil {
		return nil, err
	}

	return &musicRecordingResolver{&musicRecording}, nil
}

func pickBestValue(stringSources []StringSource) (maxScoreValue string, err error) {
	var maxScore int
	for _, source := range stringSources {
		score, err := strconv.Atoi(source.score)
		if err != nil {
			return "", err
		}
		if score > maxScore {
			maxScoreValue = source.value
		}
	}
	return
}

type InterestedPartyAgreement struct {
	agreementRoleCode     StringValue
	interestedPartyNumber IntValue
	party                 Party
	prSociety             StringValue
	prShare               IntValue
	mrSociety             StringValue
	mrShare               IntValue
}

// interestedPartyAgreementResolver defines grapQL resolver functions for the recording fields
type interestedPartyAgreementResolver struct {
	interestedPartyAgreement *InterestedPartyAgreement
}

func (i *interestedPartyAgreementResolver) AgreementRoleCode() *stringValueResolver {
	return &stringValueResolver{value: i.interestedPartyAgreement.agreementRoleCode.value, sources: i.interestedPartyAgreement.agreementRoleCode.sources}
}

func (i *interestedPartyAgreementResolver) InterestedPartyNumber() *intValueResolver {
	return &intValueResolver{value: i.interestedPartyAgreement.interestedPartyNumber.value, sources: i.interestedPartyAgreement.interestedPartyNumber.sources}
}

func (i *interestedPartyAgreementResolver) Party() *partyResolver {
	return &partyResolver{&i.interestedPartyAgreement.party}
}

func (i *interestedPartyAgreementResolver) MrSociety() *stringValueResolver {
	return &stringValueResolver{value: i.interestedPartyAgreement.mrSociety.value, sources: i.interestedPartyAgreement.mrSociety.sources}
}

func (i *interestedPartyAgreementResolver) MrShare() *intValueResolver {
	return &intValueResolver{value: i.interestedPartyAgreement.mrShare.value, sources: i.interestedPartyAgreement.mrShare.sources}
}

func (i *interestedPartyAgreementResolver) PrSociery() *stringValueResolver {
	return &stringValueResolver{value: i.interestedPartyAgreement.prSociety.value, sources: i.interestedPartyAgreement.prSociety.sources}
}

func (i *interestedPartyAgreementResolver) PrShare() *intValueResolver {
	return &intValueResolver{value: i.interestedPartyAgreement.prShare.value, sources: i.interestedPartyAgreement.prShare.sources}
}

type MusicWork struct {
	identifiers       []Identifier
	workTitle         StringValue
	territories       []TerritoryAgreement
	interestedParties []InterestedPartyAgreement
	publishers        []Party
	composers         []Composer
	recordings        []MusicRecording
}

// musicWorkResolver defines grapQL resolver functions for the recording fields
type musicWorkResolver struct {
	musicWork *MusicWork
}

func (m *musicWorkResolver) Identifiers() *[]*identifierResolver {
	var identifierResolvers []*identifierResolver
	for _, identifier := range m.musicWork.identifiers {
		identifierResolvers = append(identifierResolvers, &identifierResolver{identifier.ID, identifier.Type})
	}
	return &identifierResolvers
}

func (p *musicWorkResolver) WorkTitle() *stringValueResolver {
	return &stringValueResolver{value: p.musicWork.workTitle.value, sources: p.musicWork.workTitle.sources}
}

func (p *musicWorkResolver) Territories() *[]*territoryAgreementResolver {
	var territoryAgreementResolvers []*territoryAgreementResolver
	for _, territory := range p.musicWork.territories {
		territoryAgreementResolvers = append(territoryAgreementResolvers, &territoryAgreementResolver{territory.isoTerritoryCode, territory.condition})
	}
	return &territoryAgreementResolvers
}

func (p *musicWorkResolver) InterestedParties() *[]*territoryAgreementResolver {
	var territoryAgreementResolvers []*territoryAgreementResolver
	for _, territory := range p.musicWork.territories {
		territoryAgreementResolvers = append(territoryAgreementResolvers, &territoryAgreementResolver{territory.isoTerritoryCode, territory.condition})
	}
	return &territoryAgreementResolvers
}

type TerritoryAgreement struct {
	isoTerritoryCode StringValue
	condition        StringValue // inclusion/exclusion indicator
}

// performerResolver defines grapQL resolver functions for the recording fields
type territoryAgreementResolver struct {
	isoTerritoryCode StringValue
	condition        StringValue // inclusion/exclusion indicator
}

func (t *territoryAgreementResolver) IsoTerritoryCode() *stringValueResolver {
	return &stringValueResolver{value: t.isoTerritoryCode.value, sources: t.isoTerritoryCode.sources}
}

func (t *territoryAgreementResolver) Condition() *stringValueResolver {
	return &stringValueResolver{value: t.condition.value, sources: t.condition.sources}
}

type Composer struct {
	party Party
	works []MusicWork
}

type Performer struct {
	party      Party
	role       StringValue
	recordings []MusicRecording
	products   []MusicProduct
}

// performerResolver defines grapQL resolver functions for the recording fields
type performerResolver struct {
	performer *Performer
}

func (p *performerResolver) Role() *stringValueResolver {
	return &stringValueResolver{value: p.performer.role.value, sources: p.performer.role.sources}
}

func (p *performerResolver) Party() *partyResolver {
	return &partyResolver{&p.performer.party}
}

func (p *performerResolver) Recordings() *[]*musicRecordingResolver {
	var musicRecordingResolvers []*musicRecordingResolver
	for _, recording := range p.performer.recordings {
		musicRecordingResolvers = append(musicRecordingResolvers, &musicRecordingResolver{&recording})
	}
	return &musicRecordingResolvers
}

type Party struct {
	identifiers []Identifier
	typ         StringValue // Person || Organisation
	displayName StringValue
	formalName  StringValue
	firstName   StringValue
	lastName    StringValue
}

// performerResolver defines grapQL resolver functions for the recording fields
type partyResolver struct {
	party *Party
}

func (p *partyResolver) Identifiers() *[]*identifierResolver {
	var identifierResolvers []*identifierResolver
	for _, identifier := range p.party.identifiers {
		identifierResolvers = append(identifierResolvers, &identifierResolver{identifier.ID, identifier.Type})
	}
	return &identifierResolvers
}

func (p *partyResolver) Type() *stringValueResolver {
	return &stringValueResolver{value: p.party.typ.value, sources: p.party.typ.sources}
}

func (p *partyResolver) DisplayName() *stringValueResolver {
	return &stringValueResolver{value: p.party.displayName.value, sources: p.party.displayName.sources}
}

func (p *partyResolver) FormalName() *stringValueResolver {
	return &stringValueResolver{value: p.party.formalName.value, sources: p.party.formalName.sources}
}

func (p *partyResolver) FirstName() *stringValueResolver {
	return &stringValueResolver{value: p.party.firstName.value, sources: p.party.firstName.sources}
}

func (p *partyResolver) LastName() *stringValueResolver {
	return &stringValueResolver{value: p.party.lastName.value, sources: p.party.lastName.sources}
}

type MusicProduct struct {
	identifiers       []Identifier
	productType       []StringValue
	displayTitle      []StringValue
	genre             []StringValue
	recordings        []MusicRecording
	territoryProducts []TerritoryProduct
}

type TerritoryProduct struct {
	territories   []TerritoryAgreement
	releaseDate   StringValue
	displayTitle  StringValue
	displayArtist StringValue
	label         []Party
	genre         StringValue
}

/**
 *	MusicRecording
 */
// type MusicRecording struct {
// 	Identifiers   []Identifier `json:"identifiers, omitempty"`
// 	Title         StringValue  `json:"displayTitle, omitempty"`
// 	displayArtist []Performer
// 	contributors  []Party
// 	label         []Party
// 	Genre         StringValue `json:"genre, omitempty"`
// 	work          MusicWork
// 	// 	products:[MusicProduct]
// }
type MusicRecording struct {
	isrc string `json:"isrc, omitempty"`

	title StringValue `json:"title, omitempty"`

	products   []MusicProductLink   `json:"products, omitempty"`
	releases   []MusicReleaseLink   `json:"releases, omitempty"`
	performers []MusicPerformerLink `json:"performers, omitempty"`
	labels     []RecordLabelLink    `json:"labels, omitempty"`
	works      []MusicWorkLink      `json:"works, omitempty"`
}

// musicRecordingResolver defines grapQL resolver functions for the recording fields
type musicRecordingResolver struct {
	musicRecording *MusicRecording
}

func (r *musicRecordingResolver) Work() *musicWorkResolver {
	return &musicWorkResolver{&r.musicRecording.work}
}

func (r *musicRecordingResolver) Label() *[]*partyResolver {
	var partyResolvers []*partyResolver
	for _, party := range r.musicRecording.label {
		partyResolvers = append(partyResolvers, &partyResolver{&party})
	}
	return &partyResolvers
}

func (r *musicRecordingResolver) Contributors() *[]*partyResolver {
	var partyResolvers []*partyResolver
	for _, party := range r.musicRecording.contributors {
		partyResolvers = append(partyResolvers, &partyResolver{&party})
	}
	return &partyResolvers
}

func (r *musicRecordingResolver) DisplayArtist() *[]*performerResolver {
	var performerResolvers []*performerResolver
	for _, performer := range r.musicRecording.displayArtist {
		performerResolvers = append(performerResolvers, &performerResolver{&performer})
	}
	return &performerResolvers
}

func (r *musicRecordingResolver) DisplayTitle() *stringValueResolver {
	return &stringValueResolver{value: r.musicRecording.Title.value, sources: r.musicRecording.Title.sources}
}

func (r *musicRecordingResolver) Genre() *stringValueResolver {
	return &stringValueResolver{value: r.musicRecording.Genre.value, sources: r.musicRecording.Genre.sources}
}

func (r *musicRecordingResolver) Identifiers() *[]*identifierResolver {
	var identifierResolvers []*identifierResolver
	for _, identifier := range r.musicRecording.Identifiers {
		identifierResolvers = append(identifierResolvers, &identifierResolver{identifier.ID, identifier.Type})
	}
	return &identifierResolvers
}

// Identifier
type Identifier struct {
	ID   string `json:"id, omitempty"`
	Type string `json:"type, omitempty"`
}

// identifierResolver defines grapQL resolver functions for the identifier fields
type identifierResolver struct {
	id  string
	typ string
}

func (r *identifierResolver) ID() string {
	return r.id
}

func (r *identifierResolver) Type() string {
	return r.typ
}



// stringSourceResolver defines grapQL resolver functions for the stringSource fields
type stringSourceResolver struct {
	value  string
	source string
	score  string
}

func (ss *stringSourceResolver) Value() *string {
	return &ss.value
}

func (ss *stringSourceResolver) Source() *string {
	return &ss.source
}

func (ss *stringSourceResolver) Score() *string {
	return &ss.score
}

type StringValue struct {
	value   string         `json:"value, omitempty"`
	sources []StringSource `json:"sources, omitempty"`
}

// stringValueResolver defines grapQL resolver functions for the stringSource fields
type stringValueResolver struct {
	value   string
	sources []StringSource
}

func (sv *stringValueResolver) Value() *string {
	return &sv.value
}

func (sv *stringValueResolver) Sources() *[]*stringSourceResolver {
	var stringSourceResolvers []*stringSourceResolver
	for _, source := range sv.sources {
		stringSourceResolvers = append(stringSourceResolvers, &stringSourceResolver{source.value, source.source, source.score})
	}
	return &stringSourceResolvers
}

type IntSource struct {
	value  int    `json:"value, omitempty"`
	source string `json:"source, omitempty"`
	score  string `json:"score, omitempty"`
}

// stringSourceResolver defines grapQL resolver functions for the stringSource fields
type intSourceResolver struct {
	value  int
	source string
	score  string
}

func (is *intSourceResolver) Value() *int {
	return &is.value
}

func (is *intSourceResolver) Source() *string {
	return &is.source
}

func (is *intSourceResolver) Score() *string {
	return &is.score
}

type IntValue struct {
	value   int         `json:"value, omitempty"`
	sources []IntSource `json:"sources, omitempty"`
}

// intValueResolver defines grapQL resolver functions for the stringSource fields
type intValueResolver struct {
	value   int
	sources []IntSource
}

func (iv *intValueResolver) Value() *int {
	return &iv.value
}

func (iv *intValueResolver) Sources() *[]*intSourceResolver {
	var intSourceResolvers []*intSourceResolver
	for _, source := range iv.sources {
		intSourceResolvers = append(intSourceResolvers, &intSourceResolver{source.value, source.source, source.score})
	}
	return &intSourceResolvers
}


//
//  --- Link Types ---
//
type MusicPerformerLink struct {
  source     Source!
  performer: MusicPerformer!
  role:      String
}

type MusicComposerLink struct {
  source:   Source!
  composer: MusicComposer!
}

type RecordLabelLink struct {
  source: Source!
  label:  RecordLabel!
}

type MusicPublisherLink struct {
  source:    Source!
  publisher: MusicPublisher!
}

type MusicRecordingLink struct {
  source:    Source!
  recording: MusicRecording!
}

type MusicWorkLink struct {
  source: Source!
  work:   MusicWork!
}

type MusicReleaseLink struct {
  source:  Source!
  release: MusicRelease!
}

type MusicProductLink struct {
  source:  Source!
  product: MusicProduct!
}


//
// --- Value Types ---
//
const PartyIdentifierType (
  ISNI = 1
  IPI
  DPID
)

type PartyIdentifier  struct{
  typ   PartyIdentifierType
  value  string
  source  Source
}

type StringValue struct {
	value   string         `json:"value, omitempty"`
	sources []StringSource `json:"sources, omitempty"`
}

type StringSource struct {
	value  string `json:"value, omitempty"`
	source Source `json:"source, omitempty"`
	score  Int `json:"score, omitempty"`
}

type Source struct {
  name  string
}
