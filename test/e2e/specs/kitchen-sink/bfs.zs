function bfs(adj: i32[][], numNodes: i32, src: i32): i32[] {
    let dist: i32[] = new i32[numNodes];
    dist.fill(-1);
    dist.set(src, 0);

    let queue: i32[] = new i32[];
    queue.push(src);
    let front: i32 = 0;

    while (front < queue.length) {
        let node: i32 = queue[front];
        front = front + 1;
        let neighbors: i32[] = adj[node];
        for (let i: i32 = 0; i < neighbors.length; i++) {
            let neighbor: i32 = neighbors[i];
            if (dist[neighbor] == -1) {
                dist.set(neighbor, dist[node] + 1);
                queue.push(neighbor);
            }
        }
    }

    return dist;
}

function main(): i32 {
    let adj: i32[][] = new i32[][];
    for (let i: i32 = 0; i < 6; i++) {
        adj.push(new i32[]);
    }

    adj[0].push(1);
    adj[0].push(2);
    adj[1].push(3);
    adj[2].push(3);
    adj[3].push(4);

    let dist: i32[] = bfs(adj, 6, 0);

    if (dist[0] != 0) { return 1; }
    if (dist[1] != 1) { return 2; }
    if (dist[2] != 1) { return 3; }
    if (dist[3] != 2) { return 4; }
    if (dist[4] != 3) { return 5; }
    if (dist[5] != -1) { return 6; }

    return 0;
}
