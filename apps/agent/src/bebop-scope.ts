// Editorial questions and source locators for this selected edition. An upstream
// change that invalidates a locator blocks the build instead of moving its citation.
type Beat = [number, 'motive' | 'turn' | 'cause', string, [string, string][]]
const beats: Beat[] = [
  [
    1,
    'motive',
    '为什么飞到提华纳，双方各想得到什么？药物译为「血腥之眼」。',
    [
      ['guide', 'Tijuana asteroid colony'],
      ['official', 'ブラッディアイ”を盗み出していた'],
      ['official', '売り捌こうと'],
    ],
  ],
  [
    1,
    'turn',
    '交易为何失败？阿西莫夫怎样脱身？',
    [['guide', 'ambushed at a bar']],
  ],
  [
    1,
    'turn',
    '卡特琳娜如何介入，斯派克由此得到了什么？',
    [
      ['guide', 'before Katerina intervenes'],
      ['guide', 'Spike steals'],
    ],
  ],
  [
    1,
    'cause',
    '斯派克怎样利用那瓶药重新找到目标？交火后两人如何逼近绝境？',
    [
      ['guide', 'staged drug deal'],
      ['guide', 'interrupted by an attack'],
      ['guide', 'police blockade'],
    ],
  ],
  [
    1,
    'cause',
    '卡特琳娜在封锁线前作了什么选择？交代警方行动和追捕结果。',
    [
      ['guide', 'realizing they will never escape'],
      ['guide', 'forcing Spike to pull away'],
    ],
  ],
  [
    2,
    'motive',
    '在第二桩独立悬赏中，谁找人、谁找狗？哈基姆如何逃避辨认？',
    [
      ['guide', 'valuable lab animal'],
      ['guide', 'plastic surgery'],
      ['official', '動物研究所も'],
    ],
  ],
  [
    2,
    'turn',
    '手提箱易手如何造成认错人，为什么追人变成追狗？',
    [
      ['guide', 'briefcase containing the animal is stolen'],
      ['guide', 'assuming him to be Hakim'],
      ['guide', 'contains a Welsh Corgi'],
      ['guide', 'give chase'],
    ],
  ],
  [
    2,
    'cause',
    '杰特为什么给狗戴上项圈？解释遛狗的目的，以及研究员的哨声怎样改变计划。',
    [
      ['guide', 'gains possession of the dog'],
      ['guide', 'collar with a tracking device'],
      ['guide', 'flush out Hakim'],
      ['guide', 'activate a dog whistle'],
    ],
  ],
  [
    2,
    'turn',
    '从追车写到斯派克在赏金与救狗之间的选择，交代哈基姆的去向。',
    [
      ['guide', 'attracting Hakim'],
      ['guide', 'Hakim steals a car'],
      ['guide', 'manipulate the car door'],
      ['guide', 'lets Hakim go'],
      ['guide', 'onto a police station'],
    ],
  ],
  [
    2,
    'cause',
    '以 Bebop 号上的新成员收束。分别说明实验犬的特殊之处、谁取的名字与谁把狗留下。',
    [
      ['guide', 'genetically enhanced intellect'],
      ['guide', 'which he names'],
      ['official', '愛犬となるアイン'],
    ],
  ],
]

const thirdBeats: Beat[] = [
  [
    3,
    'motive',
    '斯派克和杰特为什么来到太空赌场？',
    [
      ['official', 'カジノにやって来た'],
      ['guide', 'space station casino'],
    ],
  ],
  [
    3,
    'turn',
    '两枚筹码怎么把普通赌客卷进了地下交易？',
    [
      ['official', '人違いから'],
      ['guide', 'mixes two poker chips'],
    ],
  ],
  [
    3,
    'turn',
    '菲为什么被追捕，她又怎么脱身？',
    [
      ['official', 'フェイの初登場話'],
      ['guide', 'wanted fugitive drowning in debt'],
    ],
  ],
  [
    3,
    'cause',
    '赌场老板戈登想拿回什么，斯派克和杰特想从他手中得到什么，交易的结局如何？',
    [
      ['official', 'ゴードンから大金'],
      ['guide', 'double-cross murder attempt'],
    ],
  ],
]
type Document = { id: string; sentences: string[] }
export function resolveBeats(documents: Document[], third = false) {
  return (third ? thirdBeats : beats).map(
    ([episode, kind, question, locators]) => ({
      episode,
      kind,
      question,
      evidence: locators.map(([source, needle]) => {
        const id = source + '-' + episode
        const doc = documents.find((d) => d.id === id)
        const matches = doc?.sentences.flatMap((text, i) =>
          text.includes(needle) ? [{ id, sentence: i + 1, text }] : [],
        )
        if (matches?.length !== 1)
          throw Error('选题的事件依据需重新核实：' + id + ' · ' + needle)
        return matches[0]
      }),
    }),
  )
}
