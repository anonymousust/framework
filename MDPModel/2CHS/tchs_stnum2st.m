function [cs,la,lh,leader] = tchs_stnum2st(num)
num = num - 1;
leader = mod(num, 2);
tmp = floor(num/2);
lh = mod(tmp, 2);
tmp2 = floor(tmp/2);
la = mod(tmp2, 2);
cs = floor(tmp2/2);
end

