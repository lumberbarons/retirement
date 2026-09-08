import { execFileSync } from "node:child_process"
import { existsSync, realpathSync, rmSync } from "node:fs"
import path from "node:path"
import { type Plugin, tool } from "@opencode-ai/plugin"

type Worktree = {
  branch: string
  path: string
  dirty: boolean
  slug: string
  baseBranch: string | null
  baseSha: string | null
  remote: string | null
  pushed: boolean
  removed?: boolean
}

const ADJECTIVES = [
  "aged", "alpine", "amber", "ancient", "aromatic", "barky", "blooming", "bosky", "brawny", "broadleaf",
  "burly", "calm", "canopied", "cool", "craggy", "dappled", "damp", "deep", "dewy", "downy",
  "dusky", "evergreen", "ferny", "feral", "fibrous", "firm", "fragrant", "frosted", "fruitful", "gnarled",
  "golden", "grassy", "green", "hardy", "hearty", "hoary", "hollow", "hushed", "knotty", "leafy",
  "lichened", "lofty", "lumber", "lush", "mighty", "misty", "mossy", "oaken", "old", "piney",
  "proud", "resinous", "rooted", "rough", "rugged", "sappy", "shady", "silvan", "solid", "spruce",
  "stalwart", "staunch", "stately", "sturdy", "sunlit", "sweet", "tall", "timber", "towering", "verdant",
  "weathered", "wild", "windy", "wiry", "woody", "young",
]

const NOUNS = [
  "acorn", "alder", "arbor", "ash", "aspen", "balsam", "bark", "baron", "beech", "birch",
  "bog", "bracken", "bramble", "branch", "briar", "brush", "bucker", "burl", "camp", "canopy",
  "canthook", "cedar", "chestnut", "choker", "clearing", "cone", "conifer", "cookhouse", "coppice", "cypress",
  "dew", "dogwood", "duff", "elder", "elm", "faller", "fern", "fir", "floor", "flume",
  "foliage", "forest", "frost", "fungus", "glade", "grove", "gum", "hazel", "hemlock", "hickory",
  "highrigger", "holly", "hollow", "jillpoke", "juniper", "kerf", "knothole", "larch", "leaf", "lichen",
  "limb", "linden", "loam", "log", "logroller", "lumber", "lumberjack", "magnate", "maple", "meadow",
  "moss", "mushroom", "needle", "oak", "peavey", "pine", "pinecone", "plane", "poplar", "raven",
  "redwood", "resin", "ridge", "ring", "river", "riverpig", "root", "rosin", "sap", "sapling",
  "sawyer", "sequoia", "shantyman", "shaving", "skidder", "spruce", "stump", "thicket", "thorn", "timber",
  "timberbeast", "topper", "trunk", "twig", "walnut", "widowmaker", "willow", "wood", "woodpile", "woods",
  "yew",
]

const fnv1a = (s: string, seed: number): number => {
  let h = seed >>> 0
  for (let i = 0; i < s.length; i++) {
    h = Math.imul(h ^ s.charCodeAt(i), 16777619) >>> 0
  }
  return h
}

export const worktreeSlug = (sessionID: string): string => {
  const h1 = fnv1a(sessionID, 0x811c9dc5)
  const h2 = fnv1a(sessionID, 0xc58f1a7b)
  const adjective = ADJECTIVES[h1 % ADJECTIVES.length]
  const noun = NOUNS[h2 % NOUNS.length]
  const number = String((h1 >>> 16) % 100).padStart(2, "0")
  return `${adjective}-${noun}-${number}`
}

export const topicSlug = (name: string): string =>
  name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40)

const isDisabled = () =>
  process.env.OPENCODE_WORKTREE_ALWAYS === "0" ||
  process.env.OPENCODE_WORKTREE_ALWAYS === "false"

const plugin: Plugin = async ({ client, directory }) => {
  const sessions = new Map<string, Worktree>()

  let repo: string | null | undefined

  const git = (args: string[], cwd: string): string | null => {
    try {
      return execFileSync("git", args, {
        cwd,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "pipe"],
      }).trim()
    } catch {
      return null
    }
  }

  const canonWalk = (p: string): string => {
    let cur = path.resolve(p)
    const tail: string[] = []
    while (!existsSync(cur)) {
      const parent = path.dirname(cur)
      if (parent === cur) break
      tail.unshift(path.basename(cur))
      cur = parent
    }
    try {
      cur = realpathSync(cur)
    } catch {}
    return tail.length ? path.join(cur, ...tail) : cur
  }

  const getRepo = (): string | null => {
    if (repo !== undefined) return repo
    if (git(["rev-parse", "--is-inside-work-tree"], directory) !== "true")
      return (repo = null)
    const commonDirRaw = git(["rev-parse", "--git-common-dir"], directory)
    if (commonDirRaw === null) return (repo = null)
    const commonDir = path.isAbsolute(commonDirRaw)
      ? commonDirRaw
      : path.resolve(directory, commonDirRaw)
    const root = canonWalk(path.dirname(commonDir))
    const topLevel = git(["rev-parse", "--show-toplevel"], directory)
    if (topLevel !== null && canonWalk(topLevel) !== root) return (repo = null)
    return (repo = root)
  }

  const listWorktrees = (root: string): { path: string; branch: string | null }[] => {
    const out: { path: string; branch: string | null }[] = []
    let last: { path: string; branch: string | null } | null = null
    for (const line of (git(["worktree", "list", "--porcelain"], root) ?? "").split("\n")) {
      if (line.startsWith("worktree ")) {
        last = { path: line.slice("worktree ".length), branch: null }
        out.push(last)
      } else if (line.startsWith("branch ") && last) {
        last.branch = line.slice("branch ".length)
      }
    }
    return out
  }

  const planWorktree = (
    sessionID: string,
  ): {
    root: string
    name: string
    branch: string
    wtPath: string
    registered: Map<string, string | null>
  } | null => {
    const root = getRepo()
    if (root === null) return null
    if (!sessionID) return null

    const registered = new Map<string, string | null>()
    for (const wt of listWorktrees(root)) {
      registered.set(wt.path, wt.branch)
    }

    const base = worktreeSlug(sessionID)
    const stored = git(["config", "--get", `opencodeWorktree.${base}`], root)
    const storedBranch =
      stored !== null &&
      git(["show-ref", "--verify", "--quiet", `refs/heads/${stored}`], root) !== null
        ? stored
        : null
    const storedDir =
      storedBranch !== null
        ? (git(["config", "--get", `opencodeWorktreePath.${base}`], root) ?? null)
        : null
    const dirName = storedDir ?? base
    let name = base
    let branch = storedBranch ?? `opencode/${base}`
    let wtPath = path.isAbsolute(dirName)
      ? dirName
      : dirName.includes(path.sep) || dirName.includes("/")
        ? path.join(root, dirName)
        : path.join(root, ".opencode", "worktrees", dirName)
    for (let i = 2; registered.has(wtPath) && registered.get(wtPath) !== `refs/heads/${branch}`; i++) {
      name = `${base}-${i}`
      branch = `opencode/${name}`
      wtPath = path.join(root, ".opencode", "worktrees", name)
    }

    return { root, name, branch, wtPath, registered }
  }

  const buildWorktree = (
    sessionID: string,
    plan: NonNullable<ReturnType<typeof planWorktree>>,
  ): Worktree => {
    const current = git(["rev-parse", "--abbrev-ref", "HEAD"], plan.root)
    const wt: Worktree = {
      branch: plan.branch,
      path: plan.wtPath,
      dirty: (git(["status", "--porcelain"], plan.root) ?? "") !== "",
      slug: plan.name,
      baseBranch: current && current !== "HEAD" ? current : null,
      baseSha: git(["rev-parse", "HEAD"], plan.root),
      remote: (git(["remote"], plan.root) ?? "").split("\n")[0]?.trim() || null,
      pushed: false,
    }
    sessions.set(sessionID, wt)
    return wt
  }

  const resolveExisting = (sessionID: string): Worktree | null => {
    const known = sessions.get(sessionID)
    if (known) return known
    const plan = planWorktree(sessionID)
    if (!plan) return null
    if (existsSync(plan.wtPath) && !plan.registered.has(plan.wtPath)) {
      rmSync(plan.wtPath, { recursive: true, force: true })
    }
    if (!existsSync(plan.wtPath) || !plan.registered.has(plan.wtPath)) return null
    return buildWorktree(sessionID, plan)
  }

  const ensureWorktree = (sessionID: string): Worktree | null => {
    const existing = resolveExisting(sessionID)
    if (existing) return existing
    const plan = planWorktree(sessionID)
    if (!plan) return null
    if (existsSync(plan.wtPath) && !plan.registered.has(plan.wtPath)) {
      rmSync(plan.wtPath, { recursive: true, force: true })
    }
    if (!existsSync(plan.wtPath)) {
      const branchExists =
        git(["show-ref", "--verify", "--quiet", `refs/heads/${plan.branch}`], plan.root) !== null
      const result = branchExists
        ? git(["worktree", "add", plan.wtPath, plan.branch], plan.root)
        : git(["worktree", "add", "-b", plan.branch, plan.wtPath, "HEAD"], plan.root)
      if (result === null) return null
    }
    return buildWorktree(sessionID, plan)
  }

  const resolveWorktree = async (
    sessionID: string,
    create: boolean,
  ): Promise<Worktree | null> => {
    const known = sessions.get(sessionID)
    if (known) return known
    let id = sessionID
    for (let depth = 0; depth < 10; depth++) {
      let parentID: string | undefined
      try {
        const res = await client.session.get({ path: { id } })
        parentID = res.data?.parentID ?? undefined
      } catch {
        const wt = create ? ensureWorktree(id) : resolveExisting(id)
        if (wt) sessions.set(sessionID, wt)
        return wt
      }
      if (!parentID) {
        const wt = create ? ensureWorktree(id) : resolveExisting(id)
        if (wt) sessions.set(sessionID, wt)
        return wt
      }
      const parentWt =
        sessions.get(parentID) ??
        (create ? ensureWorktree(parentID) : resolveExisting(parentID))
      if (parentWt) {
        sessions.set(sessionID, parentWt)
        return parentWt
      }
      id = parentID
    }
    return null
  }

  const mapPath = (p: string, wt: Worktree): string | null => {
    const root = getRepo()
    if (root === null) return null
    const raw = path.isAbsolute(p) ? path.normalize(p) : path.resolve(directory, p)
    const abs = canonWalk(raw)
    const wtAbs = canonWalk(wt.path)
    if (abs === wtAbs || abs.startsWith(wtAbs + path.sep)) return abs
    for (const other of listWorktrees(root)) {
      const otherAbs = canonWalk(other.path)
      if (otherAbs === wtAbs || otherAbs === root) continue
      if (abs === otherAbs || abs.startsWith(otherAbs + path.sep)) return abs
    }
    if (abs === root) return wt.path
    if (abs.startsWith(root + path.sep))
      return path.join(wt.path, abs.slice(root.length))
    return null
  }

  const branchTip = (wt: Worktree): string | null => {
    const root = getRepo()
    if (root === null) return null
    return git(["rev-parse", wt.branch], root)
  }

  const hasWork = (wt: Worktree): boolean => {
    if (!wt.baseSha) return false
    const tip = branchTip(wt)
    return tip !== null && tip !== wt.baseSha
  }

  type PushResult = "ok" | "merged" | "no-remote" | "no-work" | "error"

  const pushBranch = (wt: Worktree): PushResult => {
    const root = getRepo()
    if (root === null) return "error"
    if (!wt.remote) return "no-remote"
    if (!hasWork(wt)) return "no-work"
    if (wt.pushed) {
      const fetched = git(
        [
          "fetch",
          "--prune",
          wt.remote,
          `refs/heads/${wt.branch}:refs/remotes/${wt.remote}/${wt.branch}`,
        ],
        root,
      )
      if (fetched === null) return "merged"
    }
    if (git(["push", "-u", wt.remote, wt.branch], root) === null) return "error"
    wt.pushed = true
    return "ok"
  }

  const checkMerged = (wt: Worktree): boolean => {
    const root = getRepo()
    if (root === null) return false
    const custom = process.env.OPENCODE_WORKTREE_MERGED_CHECK
    if (custom) {
      try {
        execFileSync("/bin/sh", ["-c", custom], {
          cwd: root,
          env: { ...process.env, BRANCH: wt.branch, BASE: wt.baseBranch ?? "" },
          stdio: "ignore",
        })
        return true
      } catch {
        return false
      }
    }
    if (!wt.remote || !wt.baseBranch) return false
    if (git(["fetch", "--prune", wt.remote], root) === null) return false
    const remoteGone =
      git(
        ["show-ref", "--verify", "--quiet", `refs/remotes/${wt.remote}/${wt.branch}`],
        root,
      ) === null
    if (remoteGone) return true
    const tip = branchTip(wt)
    if (tip === null) return false
    return (
      git(
        ["merge-base", "--is-ancestor", tip, `refs/remotes/${wt.remote}/${wt.baseBranch}`],
        root,
      ) !== null
    )
  }

  return {
    event: async ({ event }) => {
      if (event.type === "session.idle") {
        const sessionID = (event as { properties?: { sessionID?: string } })
          .properties?.sessionID
        if (!sessionID) return
        const wt = sessions.get(sessionID)
        if (wt && !wt.removed) {
          try {
            pushBranch(wt)
          } catch {}
        }
        return
      }
      if (event.type === "session.deleted") {
        const info = (event as { properties?: { info?: { id?: string } } })
          .properties?.info
        if (info?.id) sessions.delete(info.id)
      }
    },

    "tool.execute.before": async (input, output) => {
      if (isDisabled()) return
      const mutating = input.tool === "bash" || input.tool === "write" || input.tool === "edit"
      const wt = await resolveWorktree(input.sessionID, mutating)
      if (!wt) return
      const args = (output.args ??= {})
      switch (input.tool) {
        case "bash": {
          const wd = args.workdir
          if (wd === undefined || wd === null || wd === "") {
            args.workdir = mapPath(directory, wt) ?? wt.path
          } else if (typeof wd === "string") {
            const mapped = mapPath(wd, wt)
            if (mapped !== null) args.workdir = mapped
          }
          break
        }
        case "read":
        case "write":
        case "edit": {
          const fp = args.filePath
          if (typeof fp === "string") {
            const mapped = mapPath(fp, wt)
            if (mapped !== null) args.filePath = mapped
          }
          break
        }
        case "glob":
        case "grep": {
          const p = args.path
          if (p === undefined || p === null || p === "" || p === ".") {
            args.path = mapPath(directory, wt) ?? wt.path
          } else if (typeof p === "string") {
            const mapped = mapPath(p, wt)
            if (mapped !== null) args.path = mapped
          }
          break
        }
      }
    },

    "experimental.chat.system.transform": async (input, output) => {
      if (isDisabled()) return
      if (!input.sessionID) return
      const wt = await resolveWorktree(input.sessionID, false)
      if (!wt) {
        output.system.push(
          `You are working directly in the main checkout; nothing has been changed yet.`,
          `An isolated git worktree is created automatically the first time you run a bash, write, or edit tool call inside the repo; from then on, all tool calls are transparently redirected into it and the main checkout is never modified.`,
          `read, glob, and grep never create a worktree.`,
          `If the task involves changing files or running commands, call the worktree_branch tool first with the conventional prefix for the task (feat, chore, or fix) and a short kebab-case name describing the work (e.g. prefix "feat" and name "add-dark-mode"): it creates the worktree and names the branch. Do this before the branch is pushed; the rename is refused afterwards.`,
          `If the task is to continue work that already lives in an existing worktree (e.g. re-opening the worktree for a specific issue or branch), call the worktree_switch tool with its branch or worktree name instead of creating a new worktree.`,
        )
        return
      }
      const root = getRepo()
      const lines = [
        `You are working in an isolated git worktree managed by the opencode-worktree-always plugin, not directly in the main checkout.`,
        `- Worktree path: ${wt.path}`,
        `- Branch: ${wt.branch}`,
        `- The main checkout${root ? ` (${root})` : ""} is never modified.`,
        `- All file operations (read/write/edit/glob/grep) and shell commands are transparently redirected to the worktree: use relative paths and run git commands normally; they apply to the worktree branch.`,
        `- Do not cd into the main checkout path; stay inside the worktree.`,
        `- As your very first action, call the worktree_branch tool with the conventional prefix for this task — feat for a new feature, fix for a bug fix, chore for maintenance or refactoring — and a short kebab-case name describing the work (e.g. "add-dark-mode"). The branch becomes <prefix>/<name>. Do this before the branch is pushed; the rename is refused afterwards.`,
        `- To continue work that already lives in an existing worktree (e.g. re-opening the worktree for a specific issue or branch), call the worktree_switch tool with its branch or worktree name instead; the current worktree is left in place.`,
        `- Your branch is pushed to the remote automatically after each turn.`,
        `- When you have finished the task, call the worktree_finish tool; it pushes the branch and reports the next steps.`,
        `- The normal next step after finishing is that a pull request is opened for your branch using the project's usual PR workflow; the worktree is then removed with the worktree_cleanup tool, and only after the PR is merged.`,
        `- worktree_cleanup refuses to run until the PR is merged, so do not call it early.`,
        `- After a successful worktree_cleanup, stop: the checkout no longer exists, so do not run any further file or git operations.`,
      ]
      if (wt.dirty)
        lines.push(
          `- Note: the main checkout had uncommitted changes when this worktree was created; those changes are not present in the worktree.`,
        )
      output.system.push(...lines)
    },

    tool: {
      worktree_branch: tool({
        description:
          "Create this session's git worktree if needed and rename its branch to <prefix>/<name> using the conventional prefix for the task (feat, chore, or fix) and a short descriptive name. Call this as your very first action when starting real work, before the branch is pushed.",
        args: {
          prefix: tool.schema
            .enum(["feat", "chore", "fix"])
            .describe(
              "Branch prefix for the task: feat for a new feature, fix for a bug fix, chore for maintenance or refactoring.",
            ),
          name: tool.schema
            .string()
            .optional()
            .describe(
              "Short kebab-case name describing the work (e.g. \"add-dark-mode\"). The branch becomes <prefix>/<name> and the worktree directory is renamed to match. Omit to keep the generated word slug.",
            ),
        },
        async execute(args, ctx) {
          const wt = await resolveWorktree(ctx.sessionID, true)
          if (!wt) return "No worktree for this session."
          const root = getRepo()!
          if (wt.removed) return "This session's worktree was already cleaned up."
          if (!["feat", "chore", "fix"].includes(args.prefix))
            return `Cannot rename: "${args.prefix}" is not a valid branch prefix; use feat, chore, or fix.`
          if (!wt.branch.startsWith("opencode/"))
            return `Branch is already named "${wt.branch}".`
          const pushedRemotely =
            wt.pushed ||
            (wt.remote !== null &&
              git(
                ["show-ref", "--verify", "--quiet", `refs/remotes/${wt.remote}/${wt.branch}`],
                root,
              ) !== null)
          if (pushedRemotely)
            return `Cannot rename: branch "${wt.branch}" has already been pushed to the remote. Keep the current name; call this tool as the very first action of a session instead.`
          const slug = args.name ? topicSlug(args.name) : wt.slug
          if (args.name && !slug)
            return `Cannot rename: the name "${args.name}" produces an empty slug; use letters, numbers, and hyphens.`
          const renamed = `${args.prefix}/${slug}`
          if (git(["show-ref", "--verify", "--quiet", `refs/heads/${renamed}`], root) !== null)
            return `Cannot rename to "${renamed}": a branch with that name already exists. Keep the current name "${wt.branch}".`
          const newDir =
            slug === wt.slug ? null : path.join(root, ".opencode", "worktrees", slug)
          if (
            newDir !== null &&
            (existsSync(newDir) || planWorktree(ctx.sessionID)?.registered.has(newDir))
          )
            return `Cannot rename: the worktree path ${newDir} is already taken. Pick a different name.`
          if (newDir !== null && git(["worktree", "move", wt.path, newDir], root) === null)
            return `Failed to move the worktree to ${newDir}. Keep the current name "${wt.branch}".`
          if (git(["branch", "-m", renamed], newDir ?? wt.path) === null)
            return `Failed to rename branch to "${renamed}". Keep the current name "${wt.branch}".`
          wt.branch = renamed
          git(["config", "--replace-all", `opencodeWorktree.${wt.slug}`, renamed], root)
          if (newDir !== null) {
            wt.path = newDir
            git(["config", "--replace-all", `opencodeWorktreePath.${wt.slug}`, slug], root)
          }
          return `Branch renamed to "${renamed}".`
        },
      }),

      worktree_switch: tool({
        description:
          "Switch this session to an existing git worktree so that all file operations and shell commands run in it. Use this to resume work that already lives in a worktree (e.g. when asked to re-open the worktree for a specific issue or branch) instead of creating a new one. Pass the branch name (with or without its prefix, e.g. \"feat/add-dark-mode\" or \"add-dark-mode\") or the worktree directory name; omit the argument to list the available worktrees. The worktree this session was in before switching is left in place.",
        args: {
          name: tool.schema
            .string()
            .optional()
            .describe(
              "Branch name or worktree directory name of the existing worktree to switch to (e.g. \"feat/add-dark-mode\" or \"add-dark-mode\"). Omit to list the available worktrees.",
            ),
        },
        async execute(args, ctx) {
          const root = getRepo()
          if (root === null)
            return "No worktree switching available here: this directory is not a git repository checkout."
          const others = listWorktrees(root).filter(
            (w) => w.branch !== null && canonWalk(w.path) !== root,
          )
          const describe = (ws: typeof others) =>
            ws
              .map((w) => `- ${w.branch!.replace(/^refs\/heads\//, "")} (${w.path})`)
              .join("\n")
          if (!args.name) {
            if (others.length === 0) return "No other worktrees exist yet."
            return `Existing worktrees:\n${describe(others)}`
          }
          const q = args.name.trim().replace(/^refs\/heads\//, "")
          const matches = others.filter((w) => {
            const branch = w.branch!.replace(/^refs\/heads\//, "")
            const base = path.basename(w.path)
            return branch === q || branch.endsWith(`/${q}`) || base === q
          })
          if (matches.length === 0)
            return [
              `No existing worktree matches "${args.name}".`,
              others.length === 0
                ? "There are no other worktrees yet."
                : `Available worktrees:\n${describe(others)}`,
            ].join("\n")
          if (matches.length > 1)
            return `"${args.name}" matches multiple worktrees; be more specific. Matches:\n${describe(matches)}`

          const target = matches[0]
          const current = await resolveWorktree(ctx.sessionID, false)
          if (
            current &&
            !current.removed &&
            canonWalk(current.path) === canonWalk(target.path)
          )
            return `This session is already using worktree ${current.path} (${current.branch}).`

          const branch = target.branch!.replace(/^refs\/heads\//, "")
          const wtPath = canonWalk(target.path)
          const head = git(["rev-parse", "HEAD"], root)
          const headBranch = git(["rev-parse", "--abbrev-ref", "HEAD"], root)
          const wt: Worktree = {
            branch,
            path: wtPath,
            dirty: (git(["status", "--porcelain"], target.path) ?? "") !== "",
            slug: worktreeSlug(ctx.sessionID),
            baseBranch: headBranch && headBranch !== "HEAD" ? headBranch : null,
            baseSha: head !== null ? git(["merge-base", branch, head], root) ?? head : head,
            remote: (git(["remote"], root) ?? "").split("\n")[0]?.trim() || null,
            pushed: false,
          }
          sessions.set(ctx.sessionID, wt)
          git(["config", "--replace-all", `opencodeWorktree.${wt.slug}`, branch], root)
          git(
            ["config", "--replace-all", `opencodeWorktreePath.${wt.slug}`, path.relative(root, wtPath)],
            root,
          )
          return [
            `Switched to worktree ${wt.path} (${branch}).`,
            `All file operations and shell commands now run in that worktree; the main checkout is still never modified.`,
            current
              ? `This session's previous worktree (${current.path}) was left in place.`
              : null,
          ]
            .filter((line): line is string => line !== null)
            .join("\n")
        },
      }),

      worktree_finish: tool({
        description:
          "Push this session's worktree branch to the remote and report the next steps. Call this when the task is finished.",
        args: {},
        async execute(_args, ctx) {
          const wt = await resolveWorktree(ctx.sessionID, false)
          if (!wt) return "No worktree for this session."
          if (wt.removed) return "This session's worktree was already cleaned up."
          const status: string = (() => {
            switch (pushBranch(wt)) {
              case "ok":
                return `Branch ${wt.branch} is pushed${wt.remote ? ` to ${wt.remote}` : ""} and up to date.`
              case "merged":
                return `The remote branch for "${wt.branch}" appears to have been deleted (PR merged or closed); nothing to push.`
              case "no-remote":
                return `Push not done: this repo has no git remote configured.`
              case "no-work":
                return `Branch "${wt.branch}" has no commits beyond its base yet; nothing to push.`
              case "error":
                return `Push failed; check the remote and network.`
            }
          })()
          return [
            status,
            `Worktree: ${wt.path}`,
            `The normal next step is to open a pull request for branch "${wt.branch}"${wt.baseBranch ? ` into "${wt.baseBranch}"` : ""} using the project's usual PR workflow (the plugin does not create the PR).`,
            `After the PR is merged, call worktree_cleanup to remove the worktree.`,
          ].join("\n")
        },
      }),

      worktree_cleanup: tool({
        description:
          "Remove this session's git worktree and local branch. Refuses to run until the pull request for the branch has been merged. Use force only when you are certain removal is safe.",
        args: {
          force: tool.schema
            .boolean()
            .optional()
            .describe("Remove even if the PR does not appear merged or the worktree has uncommitted changes; the local branch is kept."),
        },
        async execute(args, ctx) {
          const wt = await resolveWorktree(ctx.sessionID, false)
          if (!wt) return "No worktree for this session."
          const root = getRepo()!
          if (wt.removed) return "This session's worktree was already cleaned up."

          if ((git(["status", "--porcelain"], wt.path) ?? "") !== "" && !args.force) {
            return `Cannot clean up: the worktree at ${wt.path} has uncommitted changes. Commit (and push) them, or call with force: true to discard them.`
          }

          let merged = false
          if (!args.force) {
            merged = checkMerged(wt)
            if (!merged) {
              return [
                `Cannot clean up: the pull request for branch "${wt.branch}" does not appear to be merged yet (the remote branch still exists and its commits are not in "${wt.baseBranch ?? "the base"}").`,
                `Merge the PR first, then call worktree_cleanup again.`,
                `If you are certain it is safe (or the PR was closed rather than merged), call with force: true.`,
              ].join("\n")
            }
          }

          if (git(["worktree", "remove", "--force", wt.path], root) === null) {
            return `Failed to remove worktree at ${wt.path}. Run "git worktree remove --force ${wt.path}" manually.`
          }
          git(["worktree", "prune"], root)
          if (merged && !args.force) {
            git(["branch", "-D", wt.branch], root)
            git(["config", "--unset", `opencodeWorktree.${wt.slug}`], root)
            git(["config", "--unset", `opencodeWorktreePath.${wt.slug}`], root)
          }

          wt.removed = true
          sessions.set(ctx.sessionID, wt)

          return [
            `Worktree removed: ${wt.path}.`,
            merged
              ? `Local branch "${wt.branch}" deleted; the merged PR preserved the work.`
              : `Local branch "${wt.branch}" was kept.`,
            `Do not run any further file or git operations in this session: the checkout no longer exists. Start a new session for new work.`,
          ].join("\n")
        },
      }),
    },
  }
}

export default plugin
