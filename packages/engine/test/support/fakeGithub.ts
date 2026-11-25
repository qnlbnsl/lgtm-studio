import { execFileSync } from "node:child_process"
import { mkdirSync, writeFileSync, readFileSync } from "node:fs"
import { join } from "node:path"

/** Private executable and local bare remote. This never contacts GitHub. */
export function fakeGithub(root: string, repository: string) {
  const bin = join(root, "github-bin"), bare = join(root, "github.git"), state = join(root, "github-state.json"), calls = join(root, "github-calls.jsonl")
  mkdirSync(bin)
  const git = (...args: string[]) => execFileSync("/usr/bin/git", ["-C", repository, ...args], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim()
  execFileSync("/usr/bin/git", ["clone", "--bare", repository, bare], { stdio: "ignore" })
  const base = git("symbolic-ref", "HEAD")
  const refs = execFileSync("/usr/bin/git", ["--git-dir", bare, "for-each-ref", "--format=%(refname)", "refs/heads/"], { encoding: "utf8" }).trim().split("\n")
  for (const ref of refs) if (ref !== base) execFileSync("/usr/bin/git", ["--git-dir", bare, "update-ref", "-d", ref])
  git("remote", "add", "origin", "https://github.com/fixture/product.git")
  const gitCalls = join(root, "github-git-calls.jsonl")
  // The private executable is the test transport seam. Repository configuration
  // and get-url remain hosted URLs; only this fake transport reaches the bare repo.
  writeFileSync(join(bin, "git"), `#!${process.execPath}
const {appendFileSync}=require('node:fs'),{spawnSync,execFileSync}=require('node:child_process');
const args=process.argv.slice(2);appendFileSync(${JSON.stringify(gitCalls)},JSON.stringify(args)+'\\n');
const operation=args.findIndex(x=>x==='push'||x==='fetch');
if(operation>=0){const remote=args.slice(operation+1).find(x=>x!=='--'&&!x.startsWith('-'));const target=remote==='https://github.com/fixture/product.git'?remote:execFileSync('/usr/bin/git',['remote','get-url',...(args[operation]==='push'?['--push']:[]),remote],{encoding:'utf8'}).trim();if(target!=='https://github.com/fixture/product.git'&&!target.startsWith(${JSON.stringify(root + "/")})){process.stderr.write('Fixture refuses non-fixture transport');process.exit(128);}}
const result=spawnSync('/usr/bin/git',[...(args.includes('push')||args.includes('fetch')?['-c','protocol.file.allow=always','-c',${JSON.stringify(`url.${bare}.insteadOf=https://github.com/fixture/product.git`)}]:[]),...args],{stdio:'inherit'});process.exit(result.status??1);
`, { mode: 0o700 })
  writeFileSync(state, JSON.stringify({ state: "OPEN", mergeStateStatus: "CLEAN" }))
  writeFileSync(join(bin, "gh"), `#!${process.execPath}
const {readFileSync,writeFileSync,appendFileSync}=require('node:fs');
const {execFileSync}=require('node:child_process');
const args=process.argv.slice(2), file=${JSON.stringify(state)}, bare=${JSON.stringify(bare)};
appendFileSync(${JSON.stringify(calls)},JSON.stringify(args)+'\\n');
const s=JSON.parse(readFileSync(file,'utf8'));
const git=(...a)=>execFileSync('/usr/bin/git',['--git-dir',bare,...a],{encoding:'utf8',env:{...process.env,GIT_AUTHOR_NAME:'Fixture',GIT_AUTHOR_EMAIL:'fixture@example.invalid',GIT_COMMITTER_NAME:'Fixture',GIT_COMMITTER_EMAIL:'fixture@example.invalid'}}).trim();
const arg=(name)=>args[args.indexOf(name)+1];
if(s.hang){setInterval(()=>{},1000);return;}
if(s.overflow){process.stdout.write('x'.repeat(3*1024*1024));return;}
if(args[0]==='auth'){if(args.includes('--json')&&arg('--jq')!=='[.hosts["github.com"][]? | select(.active) | .state]')process.exit(2);if(s.offline){process.stderr.write(s.secret||'GitHub unreachable');process.exit(1);}process.stdout.write(args.includes('--json')?JSON.stringify(s.signedOut?[]:[s.authState||'success']):(s.secret||''));process.exit(s.signedOut&&!args.includes('--json')?1:0);}
const pr=(branch=s.branch,base=s.base,head)=>({isCrossRepository:false,number:17,url:'https://github.com/'+(s.repositoryName||'fixture/product')+'/pull/17',state:s.state,headRefName:branch,baseRefName:base,headRefOid:head||s.headOverride||(s.state==='MERGED'&&s.mergedHead)||git('rev-parse','refs/heads/'+branch),baseRefOid:s.creationBase||git('rev-parse','refs/heads/'+base),mergeStateStatus:s.mergeStateStatus,mergeable:s.mergeStateStatus==='DIRTY'?'CONFLICTING':'MERGEABLE',title:s.title,body:s.body,mergeCommit:s.mergeCommit?{oid:s.mergeCommit}:null});
if(args[0]==='pr'&&args[1]==='list'){const own=s.branch&&s.branch===arg('--head')&&s.base===arg('--base')&&s.state.toLowerCase()===arg('--state')?[pr()]:[];if(s.fork&&arg('--state')==='open')own.push({...pr(arg('--head'),arg('--base'),execFileSync('/usr/bin/git',['-C',${JSON.stringify(repository)},'rev-parse','refs/heads/'+arg('--head')],{encoding:'utf8'}).trim()),isCrossRepository:true,number:99,url:'https://github.com/fixture/product/pull/99'});console.log(JSON.stringify(own));return;}
if(args[0]==='pr'&&args[1]==='create'){if(s.hangAt==='create'){setInterval(()=>{},1000);return;}if(s.createRefused||(s.state==='MERGED'&&s.branch===arg('--head')&&s.mergedHead===git('rev-parse','refs/heads/'+s.branch))){process.stderr.write(s.secret||'Pull request creation refused');process.exit(1);}s.state='OPEN';s.mergeCommit=null;s.mergedHead=null;s.branch=arg('--head');s.base=arg('--base');s.creationBase=git('rev-parse','refs/heads/'+s.base);s.title=arg('--title');s.body=arg('--body');writeFileSync(file,JSON.stringify(s));console.log('https://github.com/fixture/product/pull/17');return;}
if(args[0]==='pr'&&args[1]==='edit'){if(args[2]!=='17'||arg('--body-file')!=='-')process.exit(1);s.body=readFileSync(0,'utf8');writeFileSync(file,JSON.stringify(s));return;}
if(args[0]==='pr'&&args[1]==='view'){if(/^\\d+$/.test(args[2])&&args[2]!=='17')process.exit(1);console.log(JSON.stringify(pr()));return;}
if(args[0]==='api'&&!args.includes('PUT')){const base=decodeURIComponent(args[1].split('/git/ref/heads/')[1]||'');console.log(JSON.stringify({ref:'refs/heads/'+base,object:{type:'commit',sha:git('rev-parse','refs/heads/'+base)}}));return;}\nif(args[0]==='api'&&args.includes('PUT')){
 if(s.state!=='OPEN'||s.mergeStateStatus!=='CLEAN')process.exit(1);
 const p=pr();if(!args.includes('sha='+p.headRefOid))process.exit(1);
 const liveBase=git('rev-parse','refs/heads/'+s.base);const tree=git('merge-tree','--write-tree',liveBase,p.headRefOid).split('\\n')[0];
 const sha=git('commit-tree',tree,'-p',liveBase,'-p',p.headRefOid,'-m','Fixture GitHub merge');git('update-ref','refs/heads/'+s.base,sha,liveBase);
 s.state='MERGED';s.mergeCommit=sha;s.mergedHead=p.headRefOid;writeFileSync(file,JSON.stringify(s));if(s.hangAt==='merge'){setInterval(()=>{},1000);return;}console.log(JSON.stringify({merged:true,sha}));return;
}
process.exit(1);
`, { mode: 0o700 })
  return { bin, bare, state, calls, gitCalls, set: (patch: Record<string, unknown>) => writeFileSync(state, JSON.stringify({ ...JSON.parse(readFileSync(state, "utf8")), ...patch })),
    read: () => JSON.parse(readFileSync(state, "utf8")) }
}
