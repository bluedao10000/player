// Immutable scene order within the 1998 TV pilot batch, episodes 1 and 2.
export const bebopArt = [
  {
    title: '药瓶赴约',
    episode: 1,
    brief:
      '提华纳小行星殖民地的旧街角，斯派克单手提着一小瓶密封的红色药水，正走向约定的交易地点。斑驳赭墙、低垂遮阳棚和午后硬光，安静而紧张。',
  },
  {
    title: '宠物店开箱',
    episode: 2,
    brief:
      '火星唐人街一家狭窄宠物店，柜台上的手提箱刚刚打开，一只黄白相间的短腿威尔士柯基从箱沿探出头。斯派克俯身看向箱里的狗，意外多于戒备。',
  },
  {
    title: '街头遛狗',
    episode: 2,
    brief:
      '火星唐人街街口，斯派克牵着戴追踪项圈的黄白柯基当诱饵，一边走一边扫视街角。牵绳两端与路面的倒影构成重点。',
  },
] as const

export const casinoArt = {
  title: '赌桌上的筹码',
  episode: 3,
  brief:
    '1998 TV 版太空赌场内，斯派克身处赌桌旁看着桌上的两枚外形相近的筹码，一枚牵涉秘密交易。翠绿桌毡、暖金灯光、冷紫色空间站窗口。以人物与两枚筹码为叙事重点，无需画出筹码内部，不画文字或标志。',
} as const
export const mediaAssetPattern =
  /^(?:(?:mio|bebop|bebop-portrait|bebop-e3|cfg-[1-9][0-9]*)-[1-9]-r[12](?:\.png|-crop[01]\.jpg)|bebop-reference-v1\.png)$/
