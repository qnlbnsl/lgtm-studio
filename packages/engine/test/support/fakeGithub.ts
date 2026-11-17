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
  git("config", `url.${bare}.insteadOf`, "https://github.com/fixture/product.git")
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
if(args[0]==='auth'){process.stdout.write(s.secret||'');process.exit(s.signedOut?1:0);}
const pr=()=>({number:17,url:'https://github.com/fixture/product/pull/17',state:s.state,headRefName:s.branch,baseRefName:s.base,headRefOid:git('rev-parse','refs/heads/'+s.branch),baseRefOid:git('rev-parse','refs/heads/'+s.base),mergeStateStatus:s.mergeStateStatus,mergeable:s.mergeStateStatus==='DIRTY'?'CONFLICTING':'MERGEABLE',title:s.title,body:s.body,mergeCommit:s.mergeCommit?{oid:s.mergeCommit}:null});
if(args[0]==='pr'&&args[1]==='list'){console.log(JSON.stringify(s.branch&&s.state==='OPEN'?[pr()]:[]));return;}
if(args[0]==='pr'&&args[1]==='create'){s.state='OPEN';s.mergeCommit=null;s.branch=arg('--head');s.base=arg('--base');s.title=arg('--title');s.body=arg('--body');writeFileSync(file,JSON.stringify(s));console.log('https://github.com/fixture/product/pull/17');return;}
if(args[0]==='pr'&&args[1]==='view'){console.log(JSON.stringify(pr()));return;}
if(args[0]==='api'&&args.includes('PUT')){
 if(s.state!=='OPEN'||s.mergeStateStatus!=='CLEAN')process.exit(1);
 const p=pr();if(!args.includes('sha='+p.headRefOid))process.exit(1);
 const tree=git('merge-tree','--write-tree',p.baseRefOid,p.headRefOid).split('\\n')[0];
 const sha=git('commit-tree',tree,'-p',p.baseRefOid,'-p',p.headRefOid,'-m','Fixture GitHub merge');git('update-ref','refs/heads/'+s.base,sha,p.baseRefOid);
 s.state='MERGED';s.mergeCommit=sha;writeFileSync(file,JSON.stringify(s));console.log(JSON.stringify({merged:true,sha}));return;
}
process.exit(1);
`, { mode: 0o700 })
  return { bin, bare, state, calls, set: (patch: Record<string, unknown>) => writeFileSync(state, JSON.stringify({ ...JSON.parse(readFileSync(state, "utf8")), ...patch })),
    read: () => JSON.parse(readFileSync(state, "utf8")) }
}
