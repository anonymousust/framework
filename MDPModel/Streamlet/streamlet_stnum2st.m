function [cs,la,lh,leader] = streamlet_stnum2st(num)
num = num - 1;
leader = mod(num, 2);
tmp = floor(num/2);
lh = 0;
la = mod(tmp, 2);
cs = floor(tmp/2);
end

