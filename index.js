'use strict'

const { randomBytes } = require('crypto')
const createHash = require('create-hash')
const createKeccakHash = require('keccak')
const secp256k1 = require('secp256k1')
const bs58check = require('bs58check')

function sha3 (data) {
  return createKeccakHash('keccak256').update(data).digest()
}

function ripemd160 (data) {
  return createHash('ripemd160').update(data).digest()
}

function sha2 (data) {
  return createHash('sha256').update(data).digest()
}

function byte (n) {
  return Buffer([ n ])
}

function concat (...buffers) {
  return Buffer.concat(buffers)
}

function generateSeed () {
  return randomBytes(32)
}

function derivePrivateKeys (seed) {
  if (seed.length < 32) {
    throw Error('Seed must be at least 32 bytes')
  }
  let cosmos = sha3(concat(seed, byte(0)))
  let bitcoin = sha3(concat(seed, byte(1)))
  let ethereum = sha3(concat(seed, byte(2)))
  return { cosmos, bitcoin, ethereum }
}

function derivePublicKeys (priv) {
  let cosmos = secp256k1.publicKeyCreate(priv.cosmos)
  let bitcoin = secp256k1.publicKeyCreate(priv.bitcoin)
  let ethereum = secp256k1.publicKeyCreate(priv.ethereum, false)
  return { cosmos, bitcoin, ethereum }
}

function getBitcoinAddress (pub, testnet = false) {
  let prefix = testnet ? 0x6f : 0x00
  let hash = ripemd160(sha2(pub))
  let payload = concat(byte(prefix), hash)
  return bs58check.encode(payload)
}

function getEthereumAddress (pub) {
  return '0x' + sha3(pub).slice(-20).toString('hex')
}

function getCosmosAddress (pub) {
  return ripemd160(pub).toString('hex')
}

function deriveAddresses (pub, testnet = false) {
  let cosmos = getCosmosAddress(pub.cosmos)
  let bitcoin = getBitcoinAddress(pub.bitcoin, testnet)
  let ethereum = getEthereumAddress(pub.ethereum)
  return { cosmos, bitcoin, ethereum }
}

module.exports = {
  generateSeed,
  derivePrivateKeys,
  derivePublicKeys,
  deriveAddresses
}
