import assert from 'node:assert/strict'
import test from 'node:test'
import { Readable } from 'node:stream'
import { readRequestText } from '../src/request-body.ts'

test('decode UTF-8 after joining chunks; empty input is not overflow', async () => {
  const bytes = Buffer.from('图A')
  const stream = Readable.from([bytes.subarray(0, 1), bytes.subarray(1)])
  const response = { writeHead: () => assert.fail('unexpected HTTP error') }
  assert.equal(await readRequestText(stream, response, bytes.length), '图A')
  assert.equal(await readRequestText(Readable.from([]), response, 0), '')
})
