UPDATE sets
SET
    is_standard_legal = FALSE,
    rotation_date = CASE code
        WHEN 'WOE' THEN '2027-01-23'
        WHEN 'LCI' THEN '2027-01-23'
        WHEN 'MKM' THEN '2027-01-23'
        WHEN 'OTJ' THEN '2027-01-23'
        WHEN 'BIG' THEN '2028-01-01'
        WHEN 'BLB' THEN '2028-01-01'
        WHEN 'DSK' THEN '2028-01-01'
        WHEN 'AED' THEN '2028-01-01'
        WHEN 'FDN' THEN '2029-01-01'
        WHEN 'TDM' THEN '2028-01-01'
        WHEN 'ECL' THEN '2028-01-01'
        ELSE rotation_date
    END
WHERE code IN ('WOE','LCI','MKM','OTJ','BIG','BLB','DSK','AED','FDN','TDM','ECL');
