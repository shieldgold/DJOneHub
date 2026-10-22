const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const source = fs.readFileSync('cmd/djonehub-macos/web/app.js', 'utf8');
const start = source.indexOf('$("#send-form").addEventListener("submit"');
const end = source.indexOf('$("#at-form")', start);
async function test(fail) {
 let callback, resolve, reject, calls = 0;
 const button = {textContent:'发送',disabled:false};
 const fields = {
  '#send-form': {addEventListener:(_,fn)=>callback=fn,querySelector:()=>button},
  '#send-status': {textContent:''}, '#phone':{value:'10000'}, '#message':{value:'test'},
 };
 const api = () => {calls++;return new Promise((a,b)=>{resolve=a;reject=b;});};
 vm.runInNewContext('let smsSending=false;\n'+source.slice(start,end), {$:k=>fields[k],api,notice:()=>{},AbortSignal});
 // Keyboard submission can have no submitter.
 const pending = callback({preventDefault(){},submitter:null});
 assert.equal(button.disabled,true);
 assert.match(fields['#send-status'].textContent,/发送中/);
 await callback({preventDefault(){},submitter:button});
 assert.equal(calls,1,'duplicate submission');
 if(fail)reject(new Error('模块拒绝发送：+CMS ERROR: 500'));else resolve({segments:1});
 await pending;
 assert.equal(button.disabled,false);
 assert.match(fields['#send-status'].textContent,fail?/CMS ERROR: 500/:/提交/);
 assert.equal(fields['#message'].value,fail?'test':'');
}
(async()=>{await test(false);await test(true);console.log('SMS UI tests passed');})().catch(e=>{console.error(e);process.exit(1)});
