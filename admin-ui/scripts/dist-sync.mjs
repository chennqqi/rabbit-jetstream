import {createHash} from "node:crypto";
import {copyFile, lstat, mkdir, readFile, readdir, rename, rm} from "node:fs/promises";
import {fileURLToPath} from "node:url";
import path from "node:path";

const allowedFile = /^(?:index\.html|assets\/[A-Za-z0-9_-]+\.(?:js|css))$/;

async function snapshot(root,{strict=true}={}) {
  const files=[];
  async function walk(directory) {
    for(const entry of await readdir(directory,{withFileTypes:true})) {
      const absolute=path.join(directory,entry.name), relative=path.relative(root,absolute).replaceAll("\\","/");
      const metadata=await lstat(absolute);
      if(metadata.isSymbolicLink())throw new Error(`symbolic links are not permitted: ${relative}`);
      if(metadata.isDirectory()){await walk(absolute);continue;}
      if(!metadata.isFile()||(strict&&!allowedFile.test(relative)))throw new Error(`unsupported candidate entry: ${relative}`);
      const bytes=await readFile(absolute);
      files.push({path:relative,bytes:bytes.length,sha256:createHash("sha256").update(bytes).digest("hex")});
    }
  }
  await walk(root);files.sort((a,b)=>a.path.localeCompare(b.path));return files;
}

function sameSnapshot(left,right) {
  return JSON.stringify(left)===JSON.stringify(right);
}

async function validateCandidate(candidate) {
  const files=await snapshot(candidate), names=new Set(files.map(file=>file.path));
  if(!names.has("index.html"))throw new Error("candidate is missing index.html");
  const html=await readFile(path.join(candidate,"index.html"),"utf8");
  const references=[...html.matchAll(/(?:src|href)="\/admin\/(assets\/[A-Za-z0-9_-]+\.(?:js|css))"/g)].map(match=>match[1]);
  if(!references.some(name=>name.endsWith(".js"))||!references.some(name=>name.endsWith(".css")))throw new Error("candidate entrypoint must reference JavaScript and CSS assets");
  for(const reference of references)if(!names.has(reference))throw new Error(`candidate entrypoint references missing asset: ${reference}`);
  return files;
}

async function copySnapshot(source,target,files) {
  await mkdir(target,{recursive:true});
  for(const file of files) {
    const destination=path.join(target,...file.path.split("/"));
    await mkdir(path.dirname(destination),{recursive:true});
    await copyFile(path.join(source,...file.path.split("/")),destination);
  }
}

export async function syncDistribution(candidate,dist,{check=false}={}) {
  candidate=path.resolve(candidate);dist=path.resolve(dist);
  if(candidate===dist||path.dirname(candidate)!==path.dirname(dist))throw new Error("candidate and dist must be distinct sibling directories");
  const source=await validateCandidate(candidate);
  let current;
  try{current=await snapshot(dist,{strict:false});}catch(error){if(error.code!=="ENOENT")throw error;current=[];}
  if(check) {
    if(!sameSnapshot(source,current))throw new Error("embedded dist differs from build-candidate");
    return source;
  }
  if(sameSnapshot(source,current))return source;

  const nonce=`${process.pid}-${Date.now()}`, stage=path.join(path.dirname(dist),`.dist-stage-${nonce}`), backup=path.join(path.dirname(dist),`.dist-backup-${nonce}`);
  let movedCurrent=false,movedStage=false;
  try {
    await copySnapshot(candidate,stage,source);
    if(!sameSnapshot(source,await snapshot(stage)))throw new Error("staged distribution failed content verification");
    try{await rename(dist,backup);movedCurrent=true;}catch(error){if(error.code!=="ENOENT")throw error;}
    await rename(stage,dist);movedStage=true;
    if(!sameSnapshot(source,await snapshot(dist)))throw new Error("promoted distribution failed content verification");
    if(movedCurrent)await rm(backup,{recursive:true,force:true});
    return source;
  } catch(error) {
    if(movedStage)await rm(dist,{recursive:true,force:true});
    if(movedCurrent)await rename(backup,dist);
    throw error;
  } finally {
    await rm(stage,{recursive:true,force:true});
  }
}

const invoked=fileURLToPath(import.meta.url)===path.resolve(process.argv[1]??"");
if(invoked) {
  const adminRoot=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
  const check=process.argv.slice(2).includes("--check");
  const files=await syncDistribution(path.join(adminRoot,"build-candidate"),path.join(adminRoot,"dist"),{check});
  console.log(`${check?"Verified":"Promoted"} ${files.length} embedded Admin UI files.`);
}
