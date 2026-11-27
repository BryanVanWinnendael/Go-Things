package app

// This file bridges the pure mail engine into the UI package. The app's own
// code keeps using the short, unqualified names it always has; the types and
// functions themselves live in internal/mail.

import "go-mail/internal/mail"

type (
	ThemeSettings      = mail.ThemeSettings
	Account            = mail.Account
	Attachment         = mail.Attachment
	Folder             = mail.Folder
	Message            = mail.Message
	OutgoingAttachment = mail.OutgoingAttachment
	Session            = mail.Session
	SignatureImage     = mail.SignatureImage
)

var (
	renumberSignatureAttachments = mail.RenumberSignatureAttachments
	richSignatureHTML            = mail.RichSignatureHTML
	signatureHTML                = mail.SignatureHTML
	inlineImageCount             = mail.InlineImageCount
	legacySignatureImages        = mail.LegacySignatureImages
	composeHTML                  = mail.ComposeHTML

	Open             = mail.Open
	Send             = mail.Send
	BuildMessage     = mail.BuildMessage
	BuildRichMessage = mail.BuildRichMessage

	buildMessage         = mail.BuildPlainMessage
	signatureImageParts  = mail.SignatureImageParts
	decodeSignatureImage = mail.DecodeSignatureImage
	textToHTML           = mail.TextToHTML
	folderRank           = mail.FolderRank
	encryptSecret        = mail.EncryptSecret
	decryptSecret        = mail.DecryptSecret
)
