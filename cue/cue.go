// Copyright 2016 Viacheslav Chimishuk <vchimishuk@yandex.ru>
//
// This file is part of Chub.
//
// Chub is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Chub is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Chub. If not, see <http://www.gnu.org/licenses/>.

// Package cue implement CUE-SHEET files parser.
// For CUE documentation see: http://digitalx.org/cue-sheet/syntax/
//
// TODO: Create parser specific Error (with line number and others).
package cue

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// commandParser is the function for parsing one command.
type commandParser func(params []string, sheet *Sheet) error

// commandParserDesctiptor describes command parser.
type commandParserDescriptor struct {
	// -1 -- zero or more parameters.
	paramsCount int
	parser      commandParser
}

// parsersMap used for commands and parser functions correspondence.
var parsersMap = map[string]commandParserDescriptor{
	"CATALOG":    {1, parseCatalog},
	"CDTEXTFILE": {1, parseCdTextFile},
	"FILE":       {2, parseFile},
	"FLAGS":      {-1, parseFlags},
	"INDEX":      {2, parseIndex},
	"ISRC":       {1, parseIsrc},
	"PERFORMER":  {1, parsePerformer},
	"POSTGAP":    {1, parsePostgap},
	"PREGAP":     {1, parsePregap},
	"REM":        {-1, parseRem},
	"SONGWRITER": {1, parseSongWriter},
	"TITLE":      {1, parseTitle},
	"TRACK":      {2, parseTrack},
}

const (
	// FlagTruncateStrings enables 80-char string truncation to conform the specification.
	FlagTruncateStrings = 1 << iota
)

// ParseFile parses cue-sheet tile.
func ParseFile(filename string, flags int) (sheet *Sheet, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Parse(file, flags)
}

// Parse parses cue-sheet data from reader and returns filled Sheet struct.
func Parse(reader io.Reader, flags int) (sheet *Sheet, err error) {
	sheet = &Sheet{flags: flags}

	rd := bufio.NewReader(reader)
	lineNumber := 1

	for buf, _, err := rd.ReadLine(); err != io.EOF; buf, _, err = rd.ReadLine() {
		if err != nil {
			return nil, err
		}

		line := strings.TrimSpace(string(buf))

		// Skip empty lines.
		if len(line) == 0 {
			continue
		}

		cmd, params, err := parseCommand(line)
		if err != nil {
			return nil, fmt.Errorf("Line %d. %s", lineNumber, err)
		}

		parserDescriptor, ok := parsersMap[cmd]
		if !ok {
			return nil, fmt.Errorf("Line %d. Unknown command '%s'.", lineNumber, cmd)
		}

		paramsExpected := parserDescriptor.paramsCount
		paramsRecieved := len(params)
		if paramsExpected != -1 && paramsExpected != paramsRecieved {
			return nil, fmt.Errorf("Line %d. Command %s expected %d parameters but %d received.",
				lineNumber, cmd, paramsExpected, paramsRecieved)
		}

		err = parserDescriptor.parser(params, sheet)
		if err != nil {
			return nil, fmt.Errorf("Line %d. %s", lineNumber, err)
		}

		lineNumber++
	}

	return sheet, nil
}

// parseCatalog parsers CATALOG command.
func parseCatalog(params []string, sheet *Sheet) error {
	num := params[0]

	// TODO: Optimize regexp.
	matched, _ := regexp.MatchString("^[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]$", num)
	if !matched {
		return fmt.Errorf("%s is not valid catalog number.", params)
	}

	sheet.Catalog = num

	return nil
}

// parseCdTextFile parsers CDTEXTFILE command.
func parseCdTextFile(params []string, sheet *Sheet) error {
	sheet.CdTextFile = params[0]

	return nil
}

func parseFileType(t string) (FileType, error) {
	t = strings.ToTitle(t)
	switch t {
	case "BINARY":
		return FileTypeBinary, nil
	case "MOTOROLA":
		return FileTypeMotorola, nil
	case "AIFF":
		return FileTypeAiff, nil
	case "WAVE":
		return FileTypeWave, nil
	case "MP3":
		return FileTypeMp3, nil
	default:
		return 0, fmt.Errorf("Unsupported file type %s.", t)
	}
}

// parseFile parsers FILE command.
// params[0] -- fileName
// params[1] -- fileType
func parseFile(params []string, sheet *Sheet) error {
	fileType, err := parseFileType(params[1])
	if err != nil {
		return err
	}

	sheet.Files = append(sheet.Files, &File{
		Name: params[0],
		Type: fileType,
	})

	return nil
}

// parseFlags parsers FLAGS command.
func parseFlags(params []string, sheet *Sheet) error {
	flagParser := func(flag string) (trackFlag TrackFlag, err error) {
		var flags = map[string]TrackFlag{
			"DCP":  TrackFlagDcp,
			"4CH":  TrackFlag4ch,
			"PRE":  TrackFlagPre,
			"SCMS": TrackFlagScms,
		}

		trackFlag, ok := flags[flag]
		if !ok {
			err = fmt.Errorf("Unsupported track flag %s.", flag)
		}

		return
	}

	track := getCurrentTrack(sheet)
	if track == nil {
		return errors.New("TRACK command should appears before FLAGS command.")
	}

	for _, flagStr := range params {
		flag, err := flagParser(flagStr)
		if err != nil {
			return err
		}
		track.Flags = append(track.Flags, flag)
	}

	return nil
}

// parseIndex parsers INDEX command.
func parseIndex(params []string, sheet *Sheet) error {
	min, sec, frames, err := parseTime(params[1])
	if err != nil {
		return err
	}

	number, err := strconv.Atoi(params[0])
	if err != nil {
		return err
	}

	// All index numbers must be between 0 and 99 inclusive.
	if number < 0 || number > 99 {
		return errors.New("Invalid index number value.")
	}

	track := getCurrentTrack(sheet)
	if track == nil {
		return fmt.Errorf("TRACK command expected.")
	}

	// The first index of a file must start at 00:00:00.
	// Ignore this error, some CUE sheets don't have it
	if getFileLastIndex(getCurrentFile(sheet)) == nil {
		if min+sec+frames != 0 {
			log.Printf("INDEX 00 00:00:00 missing")
		}
	}

	// This is the first track index?
	if len(track.Indexes) == 0 {
		// The first index must be 0 or 1.
		if number >= 2 {
			return errors.New("0 or 1 index number expected.")
		}
	} else {
		// All other indexes being sequential to the first one.
		numberExpected := track.Indexes[len(track.Indexes)-1].Number + 1
		if numberExpected != number {
			return fmt.Errorf("%d index number expected.", numberExpected)
		}
	}

	track.Indexes = append(track.Indexes, &Index{
		Number: number,
		Time: &Time{
			Min:    min,
			Sec:    sec,
			Frames: frames,
		},
	})

	return nil
}

// parseIsrc parsers ISRC command.
func parseIsrc(params []string, sheet *Sheet) error {
	isrc := params[0]

	track := getCurrentTrack(sheet)
	if track == nil {
		return errors.New("TRACK command expected.")
	}

	if len(track.Indexes) != 0 {
		return errors.New("ISRC command expected.")
	}

	// TODO: Shame on you for this regexp.
	re := "^[0-9a-zA-z][0-9a-zA-z][0-9a-zA-z][0-9a-zA-z][0-9a-zA-z]" +
		"[0-9][0-9][0-9][0-9][0-9][0-9][0-9]$"
	matched, _ := regexp.MatchString(re, isrc)
	if !matched {
		return fmt.Errorf("%s is not valid ISRC number.", isrc)
	}

	track.Isrc = isrc

	return nil
}

// parsePerformer parsers PERFORMER command.
func parsePerformer(params []string, sheet *Sheet) error {
	performer := sheet.stringTruncateMaybe(params[0], 80)
	track := getCurrentTrack(sheet)

	if track == nil {
		// Performer command for the CD disk.
		sheet.Performer = performer
	} else {
		// Performer command for track.
		track.Performer = performer
	}

	return nil
}

// parsePostgap parsers POSTGAP command.
func parsePostgap(params []string, sheet *Sheet) error {
	track := getCurrentTrack(sheet)
	if track == nil {
		return errors.New("TRACK command expected.")
	}

	min, sec, frames, err := parseTime(params[0])
	if err != nil {
		return err
	}

	track.Postgap = &Time{Min: min, Sec: sec, Frames: frames}

	return nil
}

// parsePregap parsers PREGAP command.
func parsePregap(params []string, sheet *Sheet) error {
	track := getCurrentTrack(sheet)
	if track == nil {
		return errors.New("TRACK command expected.")
	}

	if len(track.Indexes) != 0 {
		return errors.New("Unexpected PREGAP command.")
	}

	min, sec, frames, err := parseTime(params[0])
	if err != nil {
		return err
	}

	track.Pregap = &Time{Min: min, Sec: sec, Frames: frames}

	return nil
}

// parseRem parsers REM command.
func parseRem(params []string, sheet *Sheet) error {
	comment := strings.Join(params, " ")
	track := getCurrentTrack(sheet)

	if track == nil {
		// Comment for the CD disk.
		sheet.Comments = append(sheet.Comments, comment)
	} else {
		// Comment for the track.
		track.Comments = append(track.Comments, comment)
	}

	return nil
}

// parseSongWriter parsers SONGWRITER command.
func parseSongWriter(params []string, sheet *Sheet) error {
	songwriter := sheet.stringTruncateMaybe(params[0], 80)
	track := getCurrentTrack(sheet)

	if track == nil {
		sheet.Songwriter = songwriter
	} else {
		track.Songwriter = songwriter
	}

	return nil
}

// parseTitle parsers TITLE command.
func parseTitle(params []string, sheet *Sheet) error {
	title := sheet.stringTruncateMaybe(params[0], 80)
	track := getCurrentTrack(sheet)

	if track == nil {
		// Title for the CD disk.
		sheet.Title = title
	} else {
		// Title command for track.
		track.Title = title
	}

	return nil
}

// parseTrack parses TRACK command.
func parseTrack(params []string, sheet *Sheet) error {
	// TRACK command should be after FILE command.
	if len(sheet.Files) == 0 {
		return fmt.Errorf("Unexpected TRACK command.")
	}

	numberStr := params[0]
	dataTypeStr := params[1]

	// Type parser function.
	parseDataType := func(t string) (dataType TrackDataType, err error) {
		var types = map[string]TrackDataType{
			"AUDIO":      DataTypeAudio,
			"CDG":        DataTypeCdg,
			"MODE1/2048": DataTypeMode1_2048,
			"MODE1/2352": DataTypeMode1_2352,
			"MODE2/2336": DataTypeMode2_2336,
			"MODE2/2352": DataTypeMode2_2352,
			"CDI/2336":   DataTypeCdi_2336,
			"CDI/2352":   DataTypeCdi_2352,
		}

		dataType, ok := types[t]
		if !ok {
			err = fmt.Errorf("Unsupported track datatype %s.", t)
		}

		return
	}

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return err
	}
	if number < 1 {
		return fmt.Errorf("Bad track number value.")
	}

	dataType, err := parseDataType(dataTypeStr)
	if err != nil {
		return err
	}

	track := &Track{
		Number:   number,
		DataType: dataType,
	}

	file := sheet.Files[len(sheet.Files)-1]

	// But all track numbers after the first must be sequential.
	if len(file.Tracks) > 0 {
		if file.Tracks[len(file.Tracks)-1].Number != number-1 {
			return fmt.Errorf("Expected track number %d, but %d received.",
				number-1, number)
		}
	}

	file.Tracks = append(file.Tracks, track)

	return nil
}

// getCurrentFile returns file object started with the last FILE command.
// Returns nil if there is no any File objects.
func getCurrentFile(sheet *Sheet) *File {
	if len(sheet.Files) == 0 {
		return nil
	}

	return sheet.Files[len(sheet.Files)-1]
}

// getCurrentTrack returns current track object, which was started with last TRACK command.
// Returns nil if there is no any Track object avaliable.
func getCurrentTrack(sheet *Sheet) *Track {
	file := getCurrentFile(sheet)
	if file == nil {
		return nil
	}

	if len(file.Tracks) == 0 {
		return nil
	}

	return file.Tracks[len(file.Tracks)-1]
}

// getFileLastIndex returns last index for the given file.
// Returns nil if file has no any indexes.
func getFileLastIndex(file *File) *Index {
	for i := len(file.Tracks) - 1; i >= 0; i-- {
		track := file.Tracks[i]

		for j := len(track.Indexes) - 1; j >= 0; j-- {
			return track.Indexes[j]
		}
	}

	return nil
}
