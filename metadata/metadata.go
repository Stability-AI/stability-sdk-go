package metadata

import (
	"errors"
	"fmt"
	"github.com/dsoprea/go-exif"
	"github.com/nofeaturesonlybugs/z85"
	"github.com/stability-ai/api-interfaces/gooseai/generation"
	"github.com/stability-ai/stability-sdk-go/stability_image"
	"github.com/wbrown/gpt_bpe"
	"google.golang.org/protobuf/proto"
	"strings"
)

// decodePbTokens decodes the given `generation.Tokens` into a string. It
// assumes that the tokens are encoded using CLIP.
func decodePbTokens(pbTokens *generation.Tokens) string {
	tokens := make(gpt_bpe.Tokens, 0, len(pbTokens.Tokens))
	for _, pbToken := range pbTokens.Tokens {
		tokens = append(tokens, (gpt_bpe.Token)(pbToken.Id))
	}
	decoded := gpt_bpe.CLIPEncoder.Decode(&tokens)
	trimmedLeft := strings.TrimPrefix(decoded, "<|startoftext|>")
	trimmedRight := strings.TrimSuffix(trimmedLeft, "<|endoftext|>")
	return trimmedRight
}

// EmbedRequest takes the `rq` Request and encode it into the `img`'s
// exif data. The altered image is returned as a byte array.
//
// NOTE: Only PNG images are presently supported.
func EmbedRequest(
	rq *generation.Request,
	img *[]byte,
) (embedded *[]byte, err error) {
	encodedRq, marshalErr := proto.Marshal(rq)
	if marshalErr != nil {
		return nil, marshalErr
	}
	if len(encodedRq)%4 != 0 {
		encodedRq = append(encodedRq, make([]byte, 4-len(encodedRq)%4)...)
	}
	z85encodedRq, z85Err := z85.Encode(encodedRq)
	if z85Err != nil {
		return nil, z85Err
	}
	z85encodedRqBytes := []byte(z85encodedRq)
	var embedErr error
	embedded, embedErr = EmbedExif("ImageHistory",
		img, &z85encodedRqBytes)
	if embedErr != nil {
		return nil, embedErr
	}
	return embedded, nil
}

// tryProtobufDecode tries to decode the given byte array as a protobuf.
// If it fails, it will try to remove the last byte and try again. This is
// because the protobuf decoder will fail if there are more bytes than it
// expects, and the z85 codec will pad the bytes with null bytes.
func tryProtobufDecode(bs *[]byte, request *generation.Request) (
	unmarshalErr error) {
	decoder := proto.UnmarshalOptions{
		AllowPartial: true,
	}
	unmarshalErr = decoder.Unmarshal(*bs, request)
	for unmarshalErr != nil && len(*bs) > 1 && (*bs)[len(*bs)-1] == byte(0) {
		*bs = (*bs)[:len(*bs)-1]
		unmarshalErr = decoder.Unmarshal(*bs, request)
	}
	return unmarshalErr
}

// DecodeRequest accepts a PNG `img` in the form of a bytearray, and attempts
// to decode the embedded `Request`. If no request is found, an empty request
// is returned along with an error.
func DecodeRequest(img *[]byte) (*generation.Request, error) {
	exifEntries, exifErr := ReadExif(*img)
	if exifErr != nil {
		return nil, exifErr
	}
	if len(exifEntries) == 0 {
		return nil, errors.New("no exif entries found")
	}
	request := &generation.Request{}
	var unmarshalErr error
	if hist, ok := exifEntries["ImageHistory"]; ok {
		// Decode the z85 encoded data into a byte array
		var z85str string
		if hist.TagTypeId == exif.TypeAscii {
			z85str = (hist.Value).(string)
		} else {
			z85str = string((hist.Value).([]byte))
		}
		paddedBs, zErr := z85.Decode(z85str)
		if zErr != nil {
			return nil, fmt.Errorf("error decoding z85: %v", zErr)
		}
		// Try to decode the protobuf
		unmarshalErr = tryProtobufDecode(&paddedBs, request)
		// Decode tokens into string.
		for _, prompt := range request.GetPrompt() {
			if text := prompt.GetText(); text == "" {
				if tokens := prompt.GetTokens(); tokens != nil {
					prompt.Prompt = &generation.Prompt_Text{
						Text: decodePbTokens(tokens),
					}
				}
			}
		}
	}
	if unmarshalErr != nil {
		unmarshalErr = fmt.Errorf("error decoding protobuf: %v",
			unmarshalErr)
	}
	return request, unmarshalErr
}

func RequantizePreserveMetadata(png *[]byte) (qzd *[]byte, err error) {
	rq, decodeErr := DecodeRequest(png)
	if decodeErr != nil {
		return png, decodeErr
	} else {
		reencoded, encodeErr := stability_image.QuantizePng(png,
			4)
		if encodeErr != nil {
			return png, encodeErr
		} else {
			return EmbedRequest(rq, reencoded)
		}
	}
}
