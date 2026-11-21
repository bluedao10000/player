INSERT INTO work_scenes SELECT v,'second-view',chapter,name,place,text,block,9,evidence FROM work_scenes WHERE v=$1 AND slug='ferry-register'
