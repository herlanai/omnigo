#!/usr/bin/env python3
"""Inventory reference UI constructors; signatures only, never copy bodies.
Name mappings locate existing implementations and do not assert behavior parity.
"""
import argparse
import json
import pathlib
import re

parser = argparse.ArgumentParser()
parser.add_argument('--source', required=True, type=pathlib.Path)
args = parser.parse_args()
root = pathlib.Path(__file__).resolve().parent.parent
source = args.source / 'backend/ui'
implementations = {}
for path in sorted((root / 'omniui/baseui').glob('*.go')):
    if path.name.endswith('_test.go'):
        continue
    for name in re.findall(r'^func ([A-Z]\w*)(?:\[.*?\])?\(', path.read_text(), re.M):
        implementations[name] = str(path.relative_to(root))
aliases = {
 'Outline':'TreeOutline', 'AccordionItem':'Accordion', 'SidebarItem':'Sidebar', 'SidebarSection':'Sidebar', 'Spacer':'Column', 'Textf':'Text', 'Link':'Button', 'BackButton':'Button', 'ForwardButton':'Button', 'Fieldset':'Field', 'Themed':'NewTheme', 'Box':'Column', 'Breadcrumbs':'Breadcrumb', 'ButtonBase':'Button',
 'CheckboxBase':'Checkbox', 'CheckboxState':'MixedCheckbox', 'CheckboxStateBase':'MixedCheckbox',
 'DialogBase':'Dialog', 'Divider':'Separator', 'Modal':'Dialog', 'PrimaryButton':'Button',
 'PopoverBase':'Popover', 'PopoverMenuButton':'DropdownMenu', 'MenuButton':'DropdownMenu',
 'Radio':'Radio', 'RadioBase':'Radio', 'Segmented':'Tabs',
 'Scroll':'ScrollArea', 'ScrollBoth':'ScrollArea', 'ScrollHorizontal':'ScrollArea',
 'SliderBase':'Slider', 'StepSlider':'Slider', 'SwitchBase':'Switch',
 'TextInput':'Input', 'TextInputBase':'Input', 'TextInputBuffer':'Input', 'TextInputBufferBase':'Input',
 'TextArea':'Textarea', 'TextAreaBase':'Textarea', 'TextAreaBuffer':'Textarea', 'TextAreaBufferBase':'Textarea',
 'Split':'Resizable', 'SplitPercent':'Resizable', 'SplitVertical':'Resizable', 'SplitVerticalPercent':'Resizable',
 'ToggleBase':'Toggle', 'TooltipBase':'Tooltip', 'Tree':'FileTree', 'TreeItem':'FileTree',
 'ToastViewportBase':'Sonner', 'Autocomplete':'Combobox',
}
entries=[]
for line in (source / 'api_gen.go').read_text().splitlines():
    match=re.match(r'func (\w+)(?:\[.*?\])?\(.*\) Element \{',line)
    if not match:
        continue
    name=match.group(1)
    target=aliases.get(name,name)
    entries.append({'name':name,'signature':line.removesuffix(' {'),
        'reference':'backend/ui/api_gen.go','implementation':implementations.get(target),
        'symbol':target if target in implementations else None,
        'status':'partial' if target in implementations else 'pending',
        'evidence':'Name/adapter mapping only; consult individual capability audit and tests.'})
subpackages=[str(p.relative_to(args.source)) for p in sorted(source.iterdir()) if p.is_dir() and p.name!='testdata']
output={'schema':1,'policy':'Complete constructor inventory is separate from parity. Existing implementations are reused, never counted complete by name. Subpackage capabilities remain in reference.json and advanced.json.','constructors':entries,'subpackages':subpackages}
(root/'migration/controls/ui.json').write_text(json.dumps(output,ensure_ascii=False,indent=2)+'\n')
print(f'{len(entries)} UI constructors inventoried; {sum(e["status"]=="pending" for e in entries)} have no mapped implementation.')
