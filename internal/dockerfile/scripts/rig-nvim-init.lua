-- rig-nvim-init.lua: pre-initialises a Neovim config at image build time (and
-- again from rig-update), so the container's first `nvim` starts ready to use.
--
-- It is loaded before the user's init.lua and each step runs in its own
-- headless Neovim, in this order:
--   nvim --headless --cmd "luafile rig-nvim-init.lua" -c "lua RigNvimPlugins()" -c qa
--   nvim --headless --cmd "luafile rig-nvim-init.lua" -c "lua RigNvimMason({'Go', 'Lua'})" -c qa
--   nvim --headless --cmd "luafile rig-nvim-init.lua" -c "lua RigNvimParsers()" -c qa
-- Mason runs in a fresh Neovim after the plugins are installed, and parsers in
-- one after Mason, because configs start their parser installs on startup and
-- may need a Mason-installed tree-sitter CLI to do so.
--
-- Only a failure to install plugins fails the step: the config is unusable
-- without them. Mason packages and parsers are reported and skipped, since the
-- config can retry them itself.

local function report(msg)
  io.stdout:write("rig-nvim-init: " .. msg .. "\n")
end

local function fail(msg)
  io.stderr:write("rig-nvim-init: " .. msg .. "\n")
  vim.cmd "cquit 1"
end

-- Capture nvim-treesitter (`main` branch) install tasks. Configs call
-- require("nvim-treesitter").install(langs) on startup and discard the async
-- task it returns, so without this there is nothing to wait for.
local parser_tasks = {}

package.preload["nvim-treesitter.install"] = function(name)
  local path = vim.api.nvim_get_runtime_file("lua/nvim-treesitter/install.lua", false)[1]
  if not path then
    error("module '" .. name .. "' not found on runtimepath")
  end
  local mod = assert(loadfile(path))(name)
  local install = mod.install
  mod.install = function(...)
    local task = install(...)
    table.insert(parser_tasks, task)
    return task
  end
  return mod
end

-- RigNvimPlugins installs the config's lazy.nvim plugins at the versions in
-- its lazy-lock.json (or the latest, without one). Configs that don't use
-- lazy.nvim are left alone.
function RigNvimPlugins()
  local ok, lazy = pcall(require, "lazy")
  if not ok then
    report "lazy.nvim not in use, skipping plugin install"
    return
  end
  lazy.restore { wait = true, show = false }

  local missing = {}
  for _, plugin in ipairs(lazy.plugins()) do
    if not plugin._.installed then
      table.insert(missing, plugin.name)
    end
  end
  if #missing > 0 then
    fail("plugins failed to install: " .. table.concat(missing, ", "))
    return
  end
  report("installed " .. #lazy.plugins() .. " plugins")
end

-- RigNvimMason installs the Mason packages an NvChad config lists in
-- mason.pkgs that serve one of the given Mason languages, plus those that
-- serve no particular language (such as tree-sitter-cli). Already-installed
-- packages are reinstalled, which upgrades them.
function RigNvimMason(languages)
  local ok, nvconfig = pcall(require, "nvconfig")
  local pkgs = ok and type(nvconfig.mason) == "table" and nvconfig.mason.pkgs or nil
  if type(pkgs) ~= "table" or #pkgs == 0 then
    report "no Mason package list (NvChad mason.pkgs), skipping Mason"
    return
  end
  pcall(function()
    require("lazy").load { plugins = { "mason.nvim" } }
  end)
  local registry_ok, registry = pcall(require, "mason-registry")
  if not registry_ok then
    report "mason.nvim not available, skipping Mason"
    return
  end
  registry.refresh() -- blocks when headless

  local wanted = {}
  for _, lang in ipairs(languages) do
    wanted[lang] = true
  end
  local selected, skipped = {}, {}
  for _, name in ipairs(pkgs) do
    local found, pkg = pcall(registry.get_package, name)
    if not found then
      report("unknown Mason package " .. name .. ", skipping it")
    else
      local serves = #(pkg.spec.languages or {}) == 0
      for _, lang in ipairs(pkg.spec.languages or {}) do
        serves = serves or wanted[lang] == true
      end
      table.insert(serves and selected or skipped, pkg)
    end
  end
  if #skipped > 0 then
    local names = vim.tbl_map(function(p)
      return p.name
    end, skipped)
    report("skipping Mason packages for languages this project doesn't use: " .. table.concat(names, ", "))
  end

  local pending, failed = #selected, {}
  for _, pkg in ipairs(selected) do
    report("installing Mason package " .. pkg.name)
    pkg:install({}, function(success, err)
      if not success then
        table.insert(failed, pkg.name .. " (" .. tostring(err) .. ")")
      end
      pending = pending - 1
    end)
  end
  vim.wait(30 * 60 * 1000, function()
    return pending == 0
  end, 200)
  if pending > 0 then
    report(pending .. " Mason package(s) timed out")
  end
  for _, f in ipairs(failed) do
    report("Mason package failed, skipping it: " .. f)
  end
end

-- RigNvimParsers waits for the treesitter parser installs the config started
-- on startup. With update = true it also updates already-installed parsers.
function RigNvimParsers(opts)
  opts = opts or {}
  if opts.update then
    local ok, ts = pcall(require, "nvim-treesitter")
    if ok and type(ts.update) == "function" then
      table.insert(parser_tasks, ts.update())
    end
  end
  for _, task in ipairs(parser_tasks) do
    local ok, err = pcall(task.wait, task, 30 * 60 * 1000)
    if not ok then
      report("treesitter parsers did not all install: " .. tostring(err))
    end
  end
  report("waited for " .. #parser_tasks .. " treesitter install task(s)")
end
