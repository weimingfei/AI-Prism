package filetype

import (
	"errors"
	"testing"
)

func TestValidatePDFByMagicBytes(t *testing.T) {
	detected, err := Validate([]byte("%PDF-1.7\ncontent"), "note.pdf", KindPDF)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if detected.Kind != KindPDF || detected.ContentType != "application/pdf" {
		t.Fatalf("unexpected detection: %#v", detected)
	}
}

func TestValidateRejectsSpoofedPDFExtension(t *testing.T) {
	_, err := Validate([]byte("not a pdf"), "note.pdf", KindPDF)
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("expected ErrUnsupportedType, got %v", err)
	}
}

func TestValidateRejectsExtensionMismatch(t *testing.T) {
	_, err := Validate([]byte("%PDF-1.7\ncontent"), "note.png", KindPDF, KindImage)
	if !errors.Is(err, ErrExtensionMismatch) {
		t.Fatalf("expected ErrExtensionMismatch, got %v", err)
	}
}

func TestValidateImageAndAudioSignatures(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	if detected, err := Validate(png, "image.png", KindImage); err != nil || detected.ContentType != "image/png" {
		t.Fatalf("unexpected png detection: detected=%#v err=%v", detected, err)
	}

	mp3 := []byte{'I', 'D', '3', 0x04, 0x00, 0x00}
	if detected, err := Validate(mp3, "voice.mp3", KindAudio); err != nil || detected.ContentType != "audio/mpeg" {
		t.Fatalf("unexpected mp3 detection: detected=%#v err=%v", detected, err)
	}
}
