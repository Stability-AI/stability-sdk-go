package metadata

import (
	"bytes"
	"fmt"
	"github.com/dsoprea/go-exif"
	exif2 "github.com/dsoprea/go-exif/v2"
	exifCommon "github.com/dsoprea/go-exif/v2/common"
	log "github.com/dsoprea/go-logging"
	pngStruct "github.com/dsoprea/go-png-image-structure"
)

type IfdEntry struct {
	IfdPath     string                `json:"ifd_path"`
	FqIfdPath   string                `json:"fq_ifd_path"`
	IfdIndex    int                   `json:"ifd_index"`
	TagId       uint16                `json:"tag_id"`
	TagName     string                `json:"tag_name"`
	TagTypeId   exif.TagTypePrimitive `json:"tag_type_id"`
	TagTypeName string                `json:"tag_type_name"`
	UnitCount   uint32                `json:"unit_count"`
	Value       interface{}           `json:"value"`
	ValueString string                `json:"value_string"`
}
type IfdEntries map[string]IfdEntry

func EmbedExif(tagName string, png *[]byte, data *[]byte) (*[]byte, error) {
	pmp := pngStruct.NewPngMediaParser()

	intfc, err := pmp.ParseBytes(*png)
	if err != nil {
		return nil, err
	}

	cs := intfc.(*pngStruct.ChunkSlice)

	// Add a new tag to the additional EXIF.
	im := exif2.NewIfdMappingWithStandard()
	ti := exif2.NewTagIndex()

	ib := exif2.NewIfdBuilder(im, ti, exifCommon.IfdStandardIfdIdentity,
		exifCommon.EncodeDefaultByteOrder)

	if addErr := ib.AddStandardWithName(tagName, *data); addErr != nil {
		return nil, addErr
	}

	// Update the image.
	if exifSetErr := cs.SetExif(ib); exifSetErr != nil {
		return nil, exifSetErr
	}

	b := new(bytes.Buffer)
	if writeErr := cs.WriteTo(b); writeErr != nil {
		return nil, writeErr
	}
	imageWithEmbeddedData := b.Bytes()
	return &imageWithEmbeddedData, nil
}

func ReadExif(data []byte) (exifData IfdEntries, err error) {
	rawExif, exifErr := exif.SearchAndExtractExif(data)
	if exifErr != nil {
		return nil, exifErr
	}

	// Run the parse.
	im := exif.NewIfdMappingWithStandard()
	ti := exif.NewTagIndex()

	entries := make(IfdEntries, 0)
	visitor := func(fqIfdPath string, ifdIndex int, tagId uint16,
		tagType exif.TagType, valueContext exif.ValueContext) (err error) {
		defer func() {
			if state := recover(); state != nil {
				err = log.Wrap(state.(error))
				log.Panic(err)
			}
		}()

		ifdPath, pathErr := im.StripPathPhraseIndices(fqIfdPath)
		if pathErr != nil {
			return pathErr
		}

		it, tagErr := ti.Get(ifdPath, tagId)
		if tagErr != nil {
			if log.Is(tagErr, exif.ErrTagNotFound) {
				fmt.Printf("WARNING: Unknown tag: [%s] (%04x)\n",
					ifdPath, tagId)
				return nil
			} else {
				return tagErr
			}
		}

		valueString := ""
		var value interface{}
		if tagType.Type() == exif.TypeUndefined {
			var undefErr error
			value, undefErr = valueContext.Undefined()
			if undefErr != nil {
				if undefErr == exif.ErrUnhandledUnknownTypedTag {
					value = nil
				} else {
					return nil
				}
			}
			valueString = fmt.Sprintf("%v", value)
		} else if tagType.Type() == exif.TypeByte {
			var byteErr error
			value, byteErr = valueContext.ReadBytes()
			if byteErr != nil {
				return byteErr
			}
		}
		var formatErr error
		valueString, formatErr = valueContext.FormatFirst()
		if formatErr != nil {
			return formatErr
		}
		if tagType.Type() == exif.TypeAscii {
			value = valueString
		}

		entry := IfdEntry{
			IfdPath:     ifdPath,
			FqIfdPath:   fqIfdPath,
			IfdIndex:    ifdIndex,
			TagId:       tagId,
			TagName:     it.Name,
			TagTypeId:   tagType.Type(),
			TagTypeName: tagType.Name(),
			UnitCount:   valueContext.UnitCount(),
			Value:       value,
			ValueString: valueString,
		}
		entries[it.Name] = entry
		return nil
	}

	_, visitErr := exif.Visit(exif.IfdStandard, im, ti, rawExif, visitor)
	if visitErr != nil {
		return nil, visitErr
	}
	return entries, nil
}
