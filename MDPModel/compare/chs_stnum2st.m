function [cs,la,lh,leader] = chs_stnum2st(num)
num = num - 1;
leader = mod(num, 2);
tmp = floor(num/2);
lh = mod(tmp, 3);
tmp2 = floor(tmp/3);
la = mod(tmp2, 2);
cs = floor(tmp2/2);
end

