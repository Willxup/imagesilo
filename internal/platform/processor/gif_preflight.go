package processor

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

func preflightGIFFrames(path string, maxFrames int64) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	header := make([]byte, 13)
	if _, err := io.ReadFull(reader, header); err != nil ||
		(!bytes.Equal(header[:6], []byte("GIF87a")) && !bytes.Equal(header[:6], []byte("GIF89a"))) {
		return 0, ErrInvalidImage
	}
	if header[10]&0x80 != 0 {
		if err := discardGIFBytes(reader, int64(3<<(uint(header[10]&0x07)+1))); err != nil {
			return 0, ErrInvalidImage
		}
	}

	var frames int64
	for {
		marker, err := reader.ReadByte()
		if err != nil {
			return 0, ErrInvalidImage
		}
		switch marker {
		case 0x00:
			continue
		case 0x21:
			if _, err := reader.ReadByte(); err != nil || skipGIFSubBlocks(reader) != nil {
				return 0, ErrInvalidImage
			}
		case 0x2c:
			descriptor := make([]byte, 9)
			if _, err := io.ReadFull(reader, descriptor); err != nil {
				return 0, ErrInvalidImage
			}
			frames++
			if maxFrames > 0 && frames > maxFrames {
				return 0, ErrTooManyPixels
			}
			if descriptor[8]&0x80 != 0 {
				if err := discardGIFBytes(reader, int64(3<<(uint(descriptor[8]&0x07)+1))); err != nil {
					return 0, ErrInvalidImage
				}
			}
			if _, err := reader.ReadByte(); err != nil || skipGIFSubBlocks(reader) != nil {
				return 0, ErrInvalidImage
			}
		case 0x3b:
			if frames == 0 {
				return 0, ErrInvalidImage
			}
			return int(frames), nil
		default:
			return 0, ErrInvalidImage
		}
	}
}

func skipGIFSubBlocks(reader *bufio.Reader) error {
	for {
		size, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if size == 0 {
			return nil
		}
		if err := discardGIFBytes(reader, int64(size)); err != nil {
			return err
		}
	}
}

func discardGIFBytes(reader io.Reader, count int64) error {
	written, err := io.CopyN(io.Discard, reader, count)
	if err != nil || written != count {
		return io.ErrUnexpectedEOF
	}
	return nil
}
