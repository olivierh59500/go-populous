// Package assetimport prepares the original data needed by the Go remake from
// user-supplied Amiga disk images. It never executes or downloads game code.
package assetimport

// Fingerprint identifies one supported game resource without storing its bytes.
type Fingerprint struct {
	Name   string
	Size   int
	SHA256 string
}

// Files lists the exact resources consumed by the renderer and audio player.
// Executables, boot files and unrelated disk contents are not imported.
var Files = []Fingerprint{
	{Name: "demo.pic", Size: 32000, SHA256: "67884ff3c408cea654baedb0148cd1e57707d27e2958ebf9736d0515ecd3b884"},
	{Name: "font.dat", Size: 4400, SHA256: "cd5bedb77ad978ff40ae0e6a69b636728dc8170bfae399589ad4e697a3b313db"},
	{Name: "gmusic1", Size: 90534, SHA256: "4ae1ec26ec8c76f7e8ee48f5c56c334878e5434e91e29bfd34ead2600f573efe"},
	{Name: "gwords", Size: 79076, SHA256: "8ea349d2f50bb7f34fec8831161c0b21bc506e4a79dee42968da2fdf433890c7"},
	{Name: "land0", Size: 33714, SHA256: "4aafd9834d181e59359ec77a7ec01fb5a107738ae2a815e929641423c9c7bd19"},
	{Name: "land1", Size: 33714, SHA256: "d1232e66f77a74b06980829cb72f428c1920d1cef67b0d846c66eee803783d43"},
	{Name: "land2", Size: 33714, SHA256: "593897ae7c11450ade6e4bdb94ac7a812c8702528d7fb2e5b2f32eba71f0071b"},
	{Name: "land3", Size: 33714, SHA256: "a1c75d5248ed524c4a17e6f1168ab519a0537130bd7dc2052181552584fbe2f5"},
	{Name: "level.dat", Size: 990, SHA256: "389eddc565c25d0db48e763471cefb2fd06401cc13fa768f40a4b3912dc59401"},
	{Name: "load.pic", Size: 40064, SHA256: "028bbcdf7c4aecda1bce6e1d6aec8b88eb22a98d49d91c7a6010f4a38967a1be"},
	{Name: "lord.pic", Size: 32032, SHA256: "e6ed81379441a258846dd07f1fa646b01d267e1754ce61c6531cf53f148cbc3f"},
	{Name: "mouths.pic", Size: 5040, SHA256: "c22adb2ce0e1aff9b21f7fd01827493876dc099be83061a6bd186a49a7d8c368"},
	{Name: "qaz.pic", Size: 32000, SHA256: "c30555f7b56ba69d59bf664701f5f8da6d1e11941798ee0b2a1ad37e6a0f1053"},
	{Name: "spr_320.dat", Size: 8320, SHA256: "9e7798cb386c2425884e55b8862e619769cd2fd1fda24f7fd4c485174fcd2168"},
	{Name: "sprites0.dat", Size: 23520, SHA256: "4b1507284aad9515203a9f0e23668368788b5d3b8da401471bee2d8fb558e36f"},
}
