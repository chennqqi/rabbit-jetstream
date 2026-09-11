const labels=Object.freeze({
  en:Object.freeze({evidence:"Replica evidence",observations:"Replica observations",leader:"Leader",follower:"Follower"}),
  zh:Object.freeze({evidence:"副本观测证据",observations:"副本观测",leader:"Leader（主节点）",follower:"Follower（副本节点）"}),
});

export function replicaLabels(language){
  return language==="zh"?labels.zh:labels.en;
}

export function replicaRole(role,language){
  const localized=replicaLabels(language);
  return role==="leader"?localized.leader:role==="follower"?localized.follower:undefined;
}
