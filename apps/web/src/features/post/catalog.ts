import { storyImage } from '../../lib/images'
export type Story = {
  slug: string
  title: string
  category: string
  image: string
  alt: string
  location: string
  intro: string
  scene: string
  options: [string, string]
  replies: [string, string]
}

export const feature: Story = {
  slug: 'night-courier',
  title: '海风里的夜班邮差',
  category: '角色手记',
  image: 'mio',
  alt: '邮差阿澪抱着一摞信，站在海滨邮筒旁',
  location: '白帆港',
  intro: '在白帆港，每一封迷路的信，都有一位等它靠岸的邮差。',
  scene:
    '阿澪在红邮筒旁停下，把没有邮票的信翻给你看。“你觉得，我们该先去哪里？”',
  options: ['陪她去渡口问问', '看看信背面的鱼鳞'],
  replies: [
    '你们赶到渡口时，船长还没收起跳板。他看了一眼信封，悄悄从驾驶室取出一本旧航海日志。',
    '蓝色鱼鳞在落日中变得透明。阿澪把信举到眼前，你和她同时读出一个藏在纸纹里的地名。',
  ],
}
export const stories: Story[] = [
  {
    slug: 'last-ferry',
    title: '末班渡船',
    category: '短篇故事',
    image: 'ferry',
    alt: '夕阳下，一艘渡船停靠在白帆港码头',
    location: '旧渡口',
    intro: '潮水退去之前，码头上还有最后一位乘客。',
    scene:
      '末班船将要解缆。阿澪指着长椅下的水迹问：“刚才那双鞋，是谁放在这里的？”',
    options: ['沿着水迹寻找', '去船上核对名册'],
    replies: [
      '水迹一直延伸到售票亭。你们在窗台发现半张褪色的儿童船票，背后写着一个夏天的日期。',
      '旧名册里夹着一张儿童票存根。阿澪把日期对上潮汐表，发现今晚恰好是同一次大潮。',
    ],
  },
  {
    slug: 'unsent-letter',
    title: '没有地址的春天',
    category: '角色手记',
    image: 'letter',
    alt: '阳光下的木桌上放着一封火漆封口的旧信',
    location: '邮局分拣室',
    intro: '一封搁了三年的信，在午后的分拣台上开出花来。',
    scene:
      '午后的分拣台铺满阳光。阿澪将纸袋轻轻递过来：“你听，里面好像有什么在动。”',
    options: ['把纸袋对着光', '去看看窗台的花盆'],
    replies: [
      '光穿过牛皮纸，照出一粒种子的影子。阿澪找来放大镜，纸袋内侧竟印着花店旧址。',
      '空花盆的底座压着一枚铜钥匙。你把它递给阿澪，她认出了旧花店卷门锁的形状。',
    ],
  },
  {
    slug: 'lighthouse',
    title: '灯塔熄灭的七分钟',
    category: '剧情片段',
    image: 'light',
    alt: '夜色里，岬角的灯塔向海面投下一束金光',
    location: '北岬灯塔',
    intro: '当光束消失，整座港口第一次听见了海的回音。',
    scene:
      '北岬灯塔暗了下来。阿澪从抽屉取出一只铜哨，把它递给你：“有人在那边等回音。”',
    options: ['向海面吹两声哨', '和阿澪提灯上山'],
    replies: [
      '铜哨的回音越过防波堤。岬角的黑暗里亮起一个小小的光点，向你们慢慢画圆。',
      '石阶尽头的门开着。守塔人正在换灯芯，他指着海面说，刚刚有条船迷了航。',
    ],
  },
]
export const catalog = [feature, ...stories]
export const storyUrl = (story: Story) => '/story/' + story.slug
export const artUrl = (story: Story) => storyImage(story.image)
