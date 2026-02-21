# 权限管理

分类说明：角色、菜单、权限与资源同步

Base URL：/v1

## [POST] 创建角色

- 接口路径: POST /admin/rbac/role
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 创建新角色（可附带菜单、权限或从模板角色复制）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | body | string | 是 | 角色名称 |
| desc | body | string | 否 | 角色描述（可选） |
| menuIds | body | array<integer> | 否 | 菜单ID列表（可选） |
| permissions | body | array<object> | 否 | 权限列表（可选） |
| permissions[].resource | body | string | 是 | 资源路径 |
| permissions[].action | body | string | 是 | 操作方法 |
| copyRoleId | body | integer | 否 | 模板角色ID（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "创建成功"
}
```

## [PUT] 更新角色

- 接口路径: PUT /admin/rbac/role
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 更新角色信息（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer | 是 | 角色ID |
| name | body | string | 否 | 角色名称（可选） |
| desc | body | string | 否 | 角色描述（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```

## [GET] 角色列表

- 接口路径: GET /admin/rbac/roles
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取所有角色列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<Role> | - |
| data[].id | integer(uint) | 角色ID |
| data[].name | string | 角色名称 |
| data[].desc | string | 角色描述 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "name": "示例名称",
      "desc": "示例说明"
    }
  ],
  "msg": "获取成功"
}
```

## [DELETE] 删除角色

- 接口路径: DELETE /admin/rbac/role
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 删除角色（需管理员权限，角色下有用户时不可删除）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "删除成功"
}
```

## [POST] 分配角色

- 接口路径: POST /admin/rbac/role/assign
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 为用户分配角色（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | body | integer | 是 | 用户ID |
| roleIds | body | array<integer> | 是 | 角色ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "分配成功"
}
```

## [DELETE] 移除角色

- 接口路径: DELETE /admin/rbac/role/remove
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 移除用户的角色（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | body | integer | 是 | 用户ID |
| roleIds | body | array<integer> | 是 | 角色ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "移除成功"
}
```

## [POST] 角色继承

- 接口路径: POST /admin/rbac/role/inherit
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 角色继承父角色（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer | 是 | 角色ID |
| parentRoleId | body | integer | 是 | 父角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "继承成功"
}
```

## [DELETE] 移除角色继承

- 接口路径: DELETE /admin/rbac/role/inherit
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 移除角色继承父角色（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer | 是 | 角色ID |
| parentRoleId | body | integer | 是 | 父角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "移除成功"
}
```

## [GET] 角色继承列表

- 接口路径: GET /admin/rbac/role/inherits
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取角色继承的父角色列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | query | integer | 是 | 角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<string> | - |

响应示例:
```json
{
  "code": 0,
  "data": [
    "普通用户"
  ],
  "msg": "获取成功"
}
```

## [GET] 角色继承树

- 接口路径: GET /admin/rbac/role/inherit/tree
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取角色继承树（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<RoleInheritTreeNode> | - |
| data[].id | integer(uint) | 角色ID |
| data[].name | string | 角色名称 |
| data[].desc | string | 角色描述 |
| data[].children | array<RoleInheritTreeNode> | - |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "name": "示例名称",
      "desc": "示例说明",
      "children": [
        {
          "id": 1,
          "name": "示例名称",
          "desc": "示例说明",
          "children": [
            {
              "id": null,
              "name": null,
              "desc": null,
              "children": null
            }
          ]
        }
      ]
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 指定角色继承树

- 接口路径: GET /admin/rbac/role/inherit/tree
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取指定角色在继承树中的分支（保留该角色的完整子树，需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<RoleInheritTreeNode> | - |
| data[].id | integer(uint) | 角色ID |
| data[].name | string | 角色名称 |
| data[].desc | string | 角色描述 |
| data[].children | array<RoleInheritTreeNode> | - |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "name": "示例名称",
      "desc": "示例说明",
      "children": [
        {
          "id": 1,
          "name": "示例名称",
          "desc": "示例说明",
          "children": [
            {
              "id": null,
              "name": null,
              "desc": null,
              "children": null
            }
          ]
        }
      ]
    }
  ],
  "msg": "获取成功"
}
```

## [PUT] 更新用户角色

- 接口路径: PUT /admin/rbac/user/roles
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 替换用户的所有角色（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | body | integer | 是 | 用户ID |
| roleIds | body | array<integer> | 是 | 角色ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```

## [GET] 获取用户角色

- 接口路径: GET /admin/rbac/user/roles
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取指定用户的角色列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<Role> | - |
| data[].id | integer(uint) | 角色ID |
| data[].name | string | 角色名称 |
| data[].desc | string | 角色描述 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 1001,
      "name": "示例名称",
      "desc": "示例说明"
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 获取用户权限

- 接口路径: GET /admin/rbac/user/permissions
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取用户最终权限列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 是 | 用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<object> | - |
| data[].resource | string | - |
| data[].action | string | - |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "resource": "/common/user/info",
      "action": "GET"
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 资源列表

- 接口路径: GET /admin/rbac/resources
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取 OpenAPI 资源列表（用于权限配置）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<object> | - |
| data[].path | string | - |
| data[].method | string | - |
| data[].summary | string | - |
| data[].tags | array<string> | - |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "path": "/path/to/resource",
      "method": "GET",
      "summary": "获取用户列表",
      "tags": [
        "示例标签"
      ]
    }
  ],
  "msg": "获取成功"
}
```

## [POST] 创建菜单

- 接口路径: POST /admin/rbac/menu
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 创建新菜单（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| title | body | string | 是 | 菜单标题 |
| titleEn | body | string | 否 | 菜单英文标题（可选） |
| titleJa | body | string | 否 | 菜单日文标题（可选） |
| icon | body | string | 否 | 菜单图标SVG字符串（可选） |
| path | body | string | 否 | 路由路径（可选） |
| name | body | string | 否 | 路由名称（可选） |
| component | body | string | 否 | 前端组件路径/标识（可选） |
| keepAlive | body | boolean | 否 | 是否缓存（可选） |
| parentId | body | integer | 否 | 父菜单ID（可选） |
| sortOrder | body | integer | 否 | 排序顺序（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "创建成功"
}
```

## [PUT] 更新菜单

- 接口路径: PUT /admin/rbac/menu
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 更新菜单信息（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | body | integer | 是 | 菜单ID |
| title | body | string | 否 | 菜单标题（可选） |
| titleEn | body | string | 否 | 菜单英文标题（可选） |
| titleJa | body | string | 否 | 菜单日文标题（可选） |
| icon | body | string | 否 | 菜单图标SVG字符串（可选） |
| path | body | string | 否 | 路由路径（可选） |
| name | body | string | 否 | 路由名称（可选） |
| component | body | string | 否 | 前端组件路径/标识（可选） |
| keepAlive | body | boolean | 否 | 是否缓存（可选） |
| parentId | body | integer | 否 | 父菜单ID（可选） |
| sortOrder | body | integer | 否 | 排序顺序（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```

## [DELETE] 删除菜单

- 接口路径: DELETE /admin/rbac/menu
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 删除菜单（需管理员权限；存在子菜单时不可删除）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 菜单ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "删除成功"
}
```

## [POST] 分配菜单

- 接口路径: POST /admin/rbac/menu/assign
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 为角色分配菜单（仅直连菜单可编辑，继承菜单将被忽略；需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer | 是 | 角色ID |
| menuIds | body | array<integer> | 是 | 菜单ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "分配成功"
}
```

## [GET] 菜单列表

- 接口路径: GET /admin/rbac/menus
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取所有菜单（树形结构，需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<Menu> | - |
| data[].id | integer(uint) | 菜单ID |
| data[].title | string | 菜单标题 |
| data[].titleEn | string | 菜单英文标题 |
| data[].titleJa | string | 菜单日文标题 |
| data[].icon | string | 菜单图标SVG字符串 |
| data[].path | string | 路由路径 |
| data[].name | string | 路由名称 |
| data[].component | string | 前端组件路径/标识 |
| data[].keepAlive | boolean | 是否缓存 |
| data[].parentId | integer(uint) | 父菜单ID |
| data[].sortOrder | integer | 排序顺序 |
| data[].children | array<Menu> | 子菜单 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 10,
      "title": "示例标题",
      "titleEn": "示例标题",
      "titleJa": "示例标题",
      "icon": "user",
      "path": "/path/to/resource",
      "name": "示例名称",
      "component": "示例字符串",
      "keepAlive": true,
      "parentId": 10,
      "sortOrder": 1,
      "children": [
        {
          "id": 10,
          "title": "示例标题",
          "titleEn": "示例标题",
          "titleJa": "示例标题",
          "icon": "user",
          "path": "/path/to/resource",
          "name": "示例名称",
          "component": "示例字符串",
          "keepAlive": true,
          "parentId": 10,
          "sortOrder": 1,
          "children": [
            {
              "id": null,
              "title": null,
              "titleEn": null,
              "titleJa": null,
              "icon": null,
              "path": null,
              "name": null,
              "component": null,
              "keepAlive": null,
              "parentId": null,
              "sortOrder": null,
              "children": null
            }
          ]
        }
      ]
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 当前用户菜单

- 接口路径: GET /admin/rbac/user/menus
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取当前用户可访问的菜单（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<Menu> | - |
| data[].id | integer(uint) | 菜单ID |
| data[].title | string | 菜单标题 |
| data[].titleEn | string | 菜单英文标题 |
| data[].titleJa | string | 菜单日文标题 |
| data[].icon | string | 菜单图标SVG字符串 |
| data[].path | string | 路由路径 |
| data[].name | string | 路由名称 |
| data[].component | string | 前端组件路径/标识 |
| data[].keepAlive | boolean | 是否缓存 |
| data[].parentId | integer(uint) | 父菜单ID |
| data[].sortOrder | integer | 排序顺序 |
| data[].children | array<Menu> | 子菜单 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 1001,
      "title": "示例标题",
      "titleEn": "示例标题",
      "titleJa": "示例标题",
      "icon": "user",
      "path": "/path/to/resource",
      "name": "示例名称",
      "component": "示例字符串",
      "keepAlive": true,
      "parentId": 1001,
      "sortOrder": 1,
      "children": [
        {
          "id": 1001,
          "title": "示例标题",
          "titleEn": "示例标题",
          "titleJa": "示例标题",
          "icon": "user",
          "path": "/path/to/resource",
          "name": "示例名称",
          "component": "示例字符串",
          "keepAlive": true,
          "parentId": 1001,
          "sortOrder": 1,
          "children": [
            {
              "id": null,
              "title": null,
              "titleEn": null,
              "titleJa": null,
              "icon": null,
              "path": null,
              "name": null,
              "component": null,
              "keepAlive": null,
              "parentId": null,
              "sortOrder": null,
              "children": null
            }
          ]
        }
      ]
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 角色菜单列表

- 接口路径: GET /admin/rbac/role/menus
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 根据角色获取菜单列表（直连菜单可编辑，继承菜单只读；需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | query | integer | 是 | 角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.directMenus | array<Menu> | - |
| data.directMenus[].id | integer(uint) | 菜单ID |
| data.directMenus[].title | string | 菜单标题 |
| data.directMenus[].titleEn | string | 菜单英文标题 |
| data.directMenus[].titleJa | string | 菜单日文标题 |
| data.directMenus[].icon | string | 菜单图标SVG字符串 |
| data.directMenus[].path | string | 路由路径 |
| data.directMenus[].name | string | 路由名称 |
| data.directMenus[].component | string | 前端组件路径/标识 |
| data.directMenus[].keepAlive | boolean | 是否缓存 |
| data.directMenus[].parentId | integer(uint) | 父菜单ID |
| data.directMenus[].sortOrder | integer | 排序顺序 |
| data.directMenus[].children | array<Menu> | 子菜单 |
| data.inheritMenus | array<Menu> | - |
| data.inheritMenus[].id | integer(uint) | 菜单ID |
| data.inheritMenus[].title | string | 菜单标题 |
| data.inheritMenus[].titleEn | string | 菜单英文标题 |
| data.inheritMenus[].titleJa | string | 菜单日文标题 |
| data.inheritMenus[].icon | string | 菜单图标SVG字符串 |
| data.inheritMenus[].path | string | 路由路径 |
| data.inheritMenus[].name | string | 路由名称 |
| data.inheritMenus[].component | string | 前端组件路径/标识 |
| data.inheritMenus[].keepAlive | boolean | 是否缓存 |
| data.inheritMenus[].parentId | integer(uint) | 父菜单ID |
| data.inheritMenus[].sortOrder | integer | 排序顺序 |
| data.inheritMenus[].children | array<Menu> | 子菜单 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "directMenus": [
      {
        "id": 1,
        "title": "示例标题",
        "titleEn": "示例标题",
        "titleJa": "示例标题",
        "icon": "user",
        "path": "普通用户",
        "name": "示例名称",
        "component": "普通用户",
        "keepAlive": true,
        "parentId": 1,
        "sortOrder": 1,
        "children": [
          {
            "id": 1,
            "title": "示例标题",
            "titleEn": "示例标题",
            "titleJa": "示例标题",
            "icon": "user",
            "path": "普通用户",
            "name": "示例名称",
            "component": "普通用户",
            "keepAlive": true,
            "parentId": 1,
            "sortOrder": 1,
            "children": [
              null
            ]
          }
        ]
      }
    ],
    "inheritMenus": [
      {
        "id": 1,
        "title": "示例标题",
        "titleEn": "示例标题",
        "titleJa": "示例标题",
        "icon": "user",
        "path": "普通用户",
        "name": "示例名称",
        "component": "普通用户",
        "keepAlive": true,
        "parentId": 1,
        "sortOrder": 1,
        "children": [
          {
            "id": 1,
            "title": "示例标题",
            "titleEn": "示例标题",
            "titleJa": "示例标题",
            "icon": "user",
            "path": "普通用户",
            "name": "示例名称",
            "component": "普通用户",
            "keepAlive": true,
            "parentId": 1,
            "sortOrder": 1,
            "children": [
              null
            ]
          }
        ]
      }
    ]
  },
  "msg": "获取成功"
}
```

## [DELETE] 移除菜单

- 接口路径: DELETE /admin/rbac/menu/remove
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 移除角色的菜单（仅直连菜单可编辑，继承菜单将被忽略；需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer | 是 | 角色ID |
| menuIds | body | array<integer> | 是 | 菜单ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "移除成功"
}
```

## [POST] 分配权限

- 接口路径: POST /admin/rbac/permission
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 为角色分配API权限（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleName | body | string | 是 | 角色名称 |
| permissions | body | array<object> | 是 | 权限列表 |
| permissions[].resource | body | string | 是 | - |
| permissions[].action | body | string | 是 | - |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "添加成功"
}
```

## [DELETE] 移除权限

- 接口路径: DELETE /admin/rbac/permission
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 移除角色的API权限（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleName | body | string | 是 | 角色名称 |
| permissions | body | array<object> | 是 | 权限列表 |
| permissions[].resource | body | string | 是 | - |
| permissions[].action | body | string | 是 | - |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "移除成功"
}
```

## [PUT] 替换角色权限

- 接口路径: PUT /admin/rbac/role/permissions
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 替换指定角色的全部权限（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer | 是 | 角色ID |
| permissions | body | array<object> | 否 | 权限列表（为空则清空） |
| permissions[].resource | body | string | 是 | 资源路径 |
| permissions[].action | body | string | 是 | 操作方法 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```

## [GET] 角色权限列表

- 接口路径: GET /admin/rbac/role/permissions
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取角色的所有权限（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | query | string | 是 | 角色名称 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<array<string>> | - |

响应示例:
```json
{
  "code": 0,
  "data": [
    [
      "p",
      "role:admin",
      "/admin/user/list",
      "GET"
    ]
  ],
  "msg": "获取成功"
}
```
