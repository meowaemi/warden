#!/usr/bin/env python3
import datetime as dt
import json
import sys
import time
import uuid
import urllib.request
import urllib.error
from pathlib import Path
base=sys.argv[1].rstrip('/') if len(sys.argv)>1 else 'http://127.0.0.1:18080'
report={'simulation':True,'started_at':dt.datetime.now(dt.timezone.utc).isoformat(),'checks':[]}
def call(path,data=None):
 req=urllib.request.Request(base+path,data=None if data is None else json.dumps(data).encode(),headers={'Authorization':'Bearer warden-demo-operator','Content-Type':'application/json'})
 with urllib.request.urlopen(req,timeout=4) as r:return json.load(r)
def wait(fn,timeout=12):
 end=time.monotonic()+timeout
 while time.monotonic()<end:
  value=fn()
  if value:return value
  time.sleep(.15)
 raise AssertionError('timed out waiting for expected state')
def record(name,**details):report['checks'].append({'check':name,'passed':True,**details});print('PASS:',name,flush=True)
def command(seconds):return {'id':str(uuid.uuid4()),'site_id':'demo-site','kind':'restart_deployment','namespace':'demo','name':'demo-app','target_uid':'demo-deployment-uid','expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(seconds=seconds)).isoformat().replace('+00:00','Z')}
def statuses():return {x['id']:x['status'] for x in call('/api/commands?site_id=demo-site')}
assert call('/api/demo/link')['simulated']
site=wait(lambda:next(iter(call('/api/sites')),None))
call('/api/demo/link',{'connected':False})
old_success=site['last_success']
long=command(45);short=command(1)
assert call('/api/commands',long)['status']=='queued'
assert call('/api/commands',short)['status']=='queued'
record('commands queued while site disconnected',command_id=long['id'])
expired=wait(lambda:statuses().get(short['id'])=='expired')
stale=wait(lambda:next(iter(call('/api/sites')))['stale'])
assert statuses()[long['id']]=='queued'
record('expired command never dispatched',command_id=short['id'])
record('last observation becomes stale during link loss')
time.sleep(1.2)
before=call('/api/history?site_id=demo-site');ids_before={x['id'] for x in before}
call('/api/demo/link',{'connected':True})
wait(lambda:statuses().get(long['id'])=='applied')
wait(lambda:not next(iter(call('/api/sites')))['stale'])
after=call('/api/history?site_id=demo-site');uploaded=[x for x in after if x['id'] not in ids_before]
assert len(uploaded)>=2,'expected buffered observations from offline interval'
assert any(x['observed_at']>old_success for x in uploaded)
record('buffered offline observations uploaded after reconnection',uploaded_observations=len(uploaded))
assert call('/api/commands',long)['status']=='applied'
record('same request ID returns original applied command')
conflict=dict(long,name='another-app')
try:call('/api/commands',conflict);raise AssertionError('conflicting duplicate accepted')
except urllib.error.HTTPError as e:assert e.code==409
record('conflicting request ID rejected with HTTP 409')
assert statuses()[short['id']]=='expired'
record('non-expired command applied after reconnect; expired command stays expired')
report['finished_at']=dt.datetime.now(dt.timezone.utc).isoformat()
report['final_site']=next(iter(call('/api/sites')))
report['commands']=call('/api/commands?site_id=demo-site')
if len(sys.argv)>2:Path(sys.argv[2]).write_text(json.dumps(report,ensure_ascii=False,indent=2))
print(json.dumps({'passed':len(report['checks']),'simulation':True}))
