import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createServer } from 'node:http';
import assert from 'node:assert/strict';
import puppeteer from 'puppeteer-core';
import { chromium } from 'playwright-core';

const binary = process.env.OMNIBROWSER_BINARY;
assert(binary, 'Set OMNIBROWSER_BINARY to the built OmniBrowser executable');
const child = spawn(binary, ['serve', '--addr', '127.0.0.1:0', '--token', 'compat', '--allow-private-network'], { stdio: ['ignore', 'ignore', 'pipe'] });
const site = createServer((req, res) => {
  res.setHeader('Content-Type', 'text/html');
  res.end(`<title>Fixture</title><style>h1{color:blue}</style><h1>Hello</h1><input id="name"><button id="b" onclick="document.title=document.querySelector('#name').value">Click</button>`);
});
let startup;
try {
  const endpoint = await new Promise((resolve, reject) => {
    let log = '';
    startup = setTimeout(() => reject(new Error('CDP startup timed out: ' + log)), 10000);
    child.stderr.on('data', chunk => {
      log += chunk;
      const match = log.match(/at (127\.0\.0\.1:\d+)/);
      if (match) { clearTimeout(startup); resolve('http://' + match[1]); }
    });
    child.once('error', reject);
    child.once('exit', code => reject(new Error(`CDP exited ${code}: ${log}`)));
  });
  site.listen(0, '127.0.0.1');
  await once(site, 'listening');
  const url = `http://127.0.0.1:${site.address().port}`;
  for (const name of ['puppeteer', 'playwright']) {
    const browser = name === 'puppeteer'
      ? await puppeteer.connect({ browserURL: endpoint, headers: { Authorization: 'Bearer compat' }, defaultViewport: null, protocolTimeout: 6000 })
      : await chromium.connectOverCDP(endpoint, { headers: { Authorization: 'Bearer compat' }, timeout: 6000, noDefaults: true });
    try {
      const page = name === 'puppeteer' ? await browser.newPage() : await browser.contexts()[0].newPage();
      const requests = [], responses = [];
      page.on('request', request => requests.push(request.url()));
      page.on('response', response => responses.push(response.url()));
      const navigation = await page.goto(url, { timeout: 6000 });
      assert(navigation, 'navigation must expose its HTTP response');
      assert.equal(navigation.status(), 200);
      assert((await navigation.text()).includes('<title>Fixture</title>'));
      assert(requests.includes(url + '/'));
      assert(responses.includes(url + '/'));
      assert.equal(await page.title(), 'Fixture');
      assert.equal(await page.evaluate(() => document.querySelector('h1').textContent), 'Hello');
      await page.evaluate(() => { document.querySelector('#name').value = 'Changed'; document.querySelector('#b').click(); });
      assert.equal(await page.title(), 'Changed');
      const handle = await page.evaluateHandle(() => ({ answer: 42 }));
      assert.equal(await page.evaluate(value => value.answer, handle), 42);
      await handle.dispose();
      assert.equal(await page.evaluate(() => new Promise(resolve => setTimeout(() => resolve(42), 0))), 42);
      const session = name === 'puppeteer' ? await page.createCDPSession() : await page.context().newCDPSession(page);
      const png = Buffer.from((await session.send('Page.captureScreenshot', { format: 'png' })).data, 'base64');
      assert.equal(png.subarray(1, 4).toString(), 'PNG');
      const pdf = Buffer.from((await session.send('Page.printToPDF', { transferMode: 'ReturnAsBase64' })).data, 'base64');
      assert.equal(pdf.subarray(0, 8).toString(), '%PDF-1.4');
      await session.detach();
      await page.close();
      console.log(`${name}: navigation, title, DOM click/input, handles, promises, CDP PNG/PDF and network request/response/body passed`);
    } finally {
      if (name === 'puppeteer') await browser.disconnect(); else await browser.close();
    }
  }
} finally {
  clearTimeout(startup);
  child.kill('SIGINT');
  site.close();
}
