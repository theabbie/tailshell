package adbx

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"math/big"
)

// An adb key is a 2048-bit RSA key, fully determined by its two primes. To keep
// the sealed token (and so the MCP URL) short, only p and q (256 bytes) are
// stored; the PEM private key and adb's .pub file are rebuilt on demand.

func primesOf(privPEM []byte) (p, q []byte, err error) {
	blk, _ := pem.Decode(privPEM)
	if blk == nil {
		return nil, nil, errors.New("adb key is not PEM")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, errors.New("adb key is not RSA")
		}
		key = rk
	} else if key, err = x509.ParsePKCS1PrivateKey(blk.Bytes); err != nil {
		return nil, nil, errors.New("cannot parse adb key")
	}
	if len(key.Primes) != 2 || key.N.BitLen() != 2048 {
		return nil, nil, errors.New("unexpected adb key shape")
	}
	pad := func(b *big.Int) []byte { return b.FillBytes(make([]byte, 128)) }
	return pad(key.Primes[0]), pad(key.Primes[1]), nil
}

func keyFromPrimes(pb, qb []byte) (*rsa.PrivateKey, error) {
	p, q := new(big.Int).SetBytes(pb), new(big.Int).SetBytes(qb)
	e := big.NewInt(65537)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, big.NewInt(1)), new(big.Int).Sub(q, big.NewInt(1)))
	d := new(big.Int).ModInverse(e, phi)
	if d == nil {
		return nil, errors.New("invalid key primes")
	}
	key := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{N: new(big.Int).Mul(p, q), E: 65537},
		D:         d,
		Primes:    []*big.Int{p, q},
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	key.Precompute()
	return key, nil
}

func privPEM(key *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// androidPubFile renders adb's .pub file: base64 of Android's RSAPublicKey
// struct, followed by a comment. Layout (AOSP libcrypto_utils):
//
//	uint32 modulus length in 32-bit words (64)
//	uint32 n0inv = -(n^-1) mod 2^32
//	uint8  modulus[256], little-endian
//	uint8  rr[256] = (2^2048)^2 mod n, little-endian
//	uint32 public exponent
func androidPubFile(key *rsa.PrivateKey) []byte {
	n := key.N
	le := func(v *big.Int) []byte {
		b := v.FillBytes(make([]byte, 256))
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
		return b
	}
	two32 := new(big.Int).Lsh(big.NewInt(1), 32)
	inv := new(big.Int).ModInverse(new(big.Int).Mod(n, two32), two32)
	n0inv := uint32(new(big.Int).Sub(two32, inv).Uint64())
	rr := new(big.Int).Exp(big.NewInt(2), big.NewInt(4096), n)

	out := make([]byte, 0, 524)
	out = binary.LittleEndian.AppendUint32(out, 64)
	out = binary.LittleEndian.AppendUint32(out, n0inv)
	out = append(out, le(n)...)
	out = append(out, le(rr)...)
	out = binary.LittleEndian.AppendUint32(out, uint32(key.E))
	return []byte(base64.StdEncoding.EncodeToString(out) + " tailshell@vercel\n")
}
