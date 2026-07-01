#!/usr/bin/env node

const GRAPHQL_URL = "https://api.github.com/graphql";

const token = process.env.PROJECT_SYNC_TOKEN;
const owner = process.env.PROJECT_OWNER || "rancher";
const p59 = Number(process.env.PROJECT_59_NUMBER || "59");
const p49 = Number(process.env.PROJECT_49_NUMBER || "49");
const sliceRepo = process.env.PROJECT_59_SLICE_REPO || "rancher/virtual-clusters-ui";
const dryRun = String(process.env.DRY_RUN || "false").toLowerCase() === "true";

if (!token) {
  console.error("Missing PROJECT_SYNC_TOKEN");
  process.exit(1);
}

async function gql(query, variables = {}) {
  const res = await fetch(GRAPHQL_URL, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ query, variables }),
  });
  const json = await res.json();
  if (json.errors) throw new Error(JSON.stringify(json.errors, null, 2));
  return json.data;
}

const norm = (s) => (s || "").trim().toLowerCase();

async function getProjectMeta(org, number) {
  const data = await gql(
    `
    query($org: String!, $number: Int!) {
      organization(login: $org) {
        projectV2(number: $number) {
          id
          title
          fields(first: 50) {
            nodes {
              ... on ProjectV2SingleSelectField {
                id
                name
                options { id name }
              }
            }
          }
        }
      }
    }
    `,
    { org, number }
  );

  const project = data.organization?.projectV2;
  if (!project) throw new Error(`Project not found: ${org} #${number}`);

  const statusField = project.fields.nodes.find((f) => norm(f.name) === "status");
  if (!statusField) throw new Error(`No Status field in ${org} #${number}`);

  return {
    projectId: project.id,
    title: project.title,
    statusFieldId: statusField.id,
    statusOptions: statusField.options.map((o) => ({ id: o.id, name: o.name })),
  };
}

async function getAllItems(projectId) {
  const out = [];
  let after = null;

  while (true) {
    const data = await gql(
      `
      query($projectId: ID!, $after: String) {
        node(id: $projectId) {
          ... on ProjectV2 {
            items(first: 100, after: $after) {
              pageInfo { hasNextPage endCursor }
              nodes {
                id
                updatedAt
                content {
                  __typename
                  ... on Issue { id url repository { nameWithOwner } }
                  ... on PullRequest { id url repository { nameWithOwner } }
                }
                fieldValues(first: 50) {
                  nodes {
                    ... on ProjectV2ItemFieldSingleSelectValue {
                      field { ... on ProjectV2SingleSelectField { id name } }
                      optionId
                    }
                  }
                }
              }
            }
          }
        }
      }
      `,
      { projectId, after }
    );

    const conn = data.node.items;
    out.push(...conn.nodes);
    if (!conn.pageInfo.hasNextPage) break;
    after = conn.pageInfo.endCursor;
  }
  return out;
}

function extractStatusOptionId(item, statusFieldId) {
  const v = item.fieldValues.nodes.find((n) => n?.field?.id === statusFieldId);
  return v?.optionId || null;
}

function buildIndex(items, { filterRepo = null } = {}) {
  const byContentId = new Map();
  const byUrl = new Map();

  for (const item of items) {
    const c = item.content;
    if (!c) continue;
    if (!(c.__typename === "Issue" || c.__typename === "PullRequest")) continue;
    if (filterRepo && c.repository?.nameWithOwner !== filterRepo) continue;

    if (c.id) byContentId.set(c.id, item);
    if (c.url) byUrl.set(c.url, item);
  }

  return { byContentId, byUrl };
}

function toMs(ts) {
  const v = Date.parse(ts || "");
  return Number.isNaN(v) ? 0 : v;
}

function makeExactNameMapper(sourceOptions, targetOptions) {
  const targetByName = new Map(targetOptions.map((o) => [norm(o.name), o.id]));
  const sourceIdToName = new Map(sourceOptions.map((o) => [o.id, o.name]));

  return (sourceOptionId) => {
    const sourceName = sourceIdToName.get(sourceOptionId);
    if (!sourceName) return null;
    return targetByName.get(norm(sourceName)) || null;
  };
}

async function updateStatus({ projectId, itemId, fieldId, optionId }) {
  if (dryRun) {
    console.log(`[DRY_RUN] ${itemId} -> ${optionId}`);
    return;
  }

  await gql(
    `
    mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!, $optionId: String!) {
      updateProjectV2ItemFieldValue(input: {
        projectId: $projectId
        itemId: $itemId
        fieldId: $fieldId
        value: { singleSelectOptionId: $optionId }
      }) {
        projectV2Item { id }
      }
    }
    `,
    { projectId, itemId, fieldId, optionId }
  );
}

(async () => {
  console.log(`Sync start: ${owner} #${p59} ↔ #${p49}`);

  const [proj59, proj49] = await Promise.all([
    getProjectMeta(owner, p59),
    getProjectMeta(owner, p49),
  ]);

  const [items59, items49] = await Promise.all([
    getAllItems(proj59.projectId),
    getAllItems(proj49.projectId),
  ]);

  const idx59 = buildIndex(items59, { filterRepo: sliceRepo });
  const idx49 = buildIndex(items49);

  const map59to49 = makeExactNameMapper(proj59.statusOptions, proj49.statusOptions);
  const map49to59 = makeExactNameMapper(proj49.statusOptions, proj59.statusOptions);

  const keys = new Set([
    ...idx59.byContentId.keys(),
    ...idx49.byContentId.keys(),
    ...idx59.byUrl.keys(),
    ...idx49.byUrl.keys(),
  ]);

  let changes = 0;

  for (const key of keys) {
    const a = idx59.byContentId.get(key) || idx59.byUrl.get(key) || null; // project 59
    const b = idx49.byContentId.get(key) || idx49.byUrl.get(key) || null; // project 49
    if (!a || !b) continue;

    const aStatus = extractStatusOptionId(a, proj59.statusFieldId);
    const bStatus = extractStatusOptionId(b, proj49.statusFieldId);

    if (!aStatus && !bStatus) continue;

    const aUpdated = toMs(a.updatedAt);
    const bUpdated = toMs(b.updatedAt);

    if (aUpdated >= bUpdated) {
      const mapped = aStatus ? map59to49(aStatus) : null;
      if (mapped && mapped !== bStatus) {
        await updateStatus({
          projectId: proj49.projectId,
          itemId: b.id,
          fieldId: proj49.statusFieldId,
          optionId: mapped,
        });
        changes++;
      }
    } else {
      const mapped = bStatus ? map49to59(bStatus) : null;
      if (mapped && mapped !== aStatus) {
        await updateStatus({
          projectId: proj59.projectId,
          itemId: a.id,
          fieldId: proj59.statusFieldId,
          optionId: mapped,
        });
        changes++;
      }
    }
  }

  console.log(`Sync complete. Changes: ${changes}`);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
