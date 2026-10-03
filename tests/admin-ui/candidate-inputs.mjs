import {lstat, readdir, readFile} from "node:fs/promises";
import path from "node:path";
import {createHash} from "node:crypto";

// Include lazy chunks and other emitted assets, not only HTML entry points.
// Reject links rather than fingerprinting files outside the candidate tree.
export async function snapshotCandidate(directory) {
  const root=path.resolve(directory),files=[];
  async function visit(file) {
    const stat=await lstat(file);
    if(stat.isSymbolicLink())throw new Error("Candidate inputs must not contain symbolic links");
    if(stat.isDirectory()) {
      for(const name of (await readdir(file)).sort())await visit(path.join(file,name));
    } else if(stat.isFile()) {
      const bytes=await readFile(file);
      files.push({path:path.relative(root,file).replaceAll("\\","/"),bytes:bytes.length,sha256:createHash("sha256").update(bytes).digest("hex")});
    } else throw new Error("Candidate input is not a regular file or directory");
  }
  await visit(root);
  if(!files.some(file=>file.path==="index.html"))throw new Error("Candidate index.html is required");
  return files;
}
