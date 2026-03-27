export type AutoActionsOptions = { associate?: boolean; clean?: boolean };

export type AutoActionsFlags = {
  autoAssociateOnAddEnabled: boolean;
  autoCleanOnDeleteEnabled: boolean;
};

export function buildAutoActionsDescription(options: AutoActionsOptions, flags: AutoActionsFlags): string | undefined {
  const actions: string[] = [];

  if (options.associate && flags.autoAssociateOnAddEnabled) {
    actions.push("自动关联");
  }

  if (options.clean && flags.autoCleanOnDeleteEnabled) {
    actions.push("自动清理无效关联");
  }

  if (actions.length === 0) return undefined;
  return `已触发${actions.join("、")}（后台异步）`;
}

